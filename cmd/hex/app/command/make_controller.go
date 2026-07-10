package command

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

// controllerAction describes one RESTful action: which HTTP verb + URL
// suffix it uses and which method name it maps to on the controller.
type controllerAction struct {
	Method string // Go method name on the controller struct (pascalCase)
	Verb   string // HTTP verb: GET, POST, PUT, DELETE
	Suffix string // path suffix appended to the resource base: "", "/:id"
}

// allControllerActions lists every RESTful action a controller can
// scaffold. Order matters for output; scaffolder emits routes in this
// order.
var allControllerActions = []struct {
	Name   string
	Action controllerAction
}{
	{"index", controllerAction{Method: "Index", Verb: "GET", Suffix: ""}},
	{"show", controllerAction{Method: "Show", Verb: "GET", Suffix: "/:id"}},
	{"store", controllerAction{Method: "Store", Verb: "POST", Suffix: ""}},
	{"update", controllerAction{Method: "Update", Verb: "PUT", Suffix: "/:id"}},
	{"destroy", controllerAction{Method: "Destroy", Verb: "DELETE", Suffix: "/:id"}},
}

// controllerData feeds the controller template.
type controllerData struct {
	// Package is the lower-case file/package-relative name (e.g. "users").
	Package string
	// Struct is the PascalCase struct name (e.g. "Users").
	Struct string
	// Constructor is the pascalCase constructor name (e.g. "NewUsers").
	Constructor string
	// Path is the URL base path used in routes (e.g. "/users").
	Path string
	// Variable is the camelCase local variable name used in routes.go
	// (e.g. "users").
	Variable string
	// Actions is the list of methods to generate (Index, Show, ...).
	Actions []controllerAction
}

func newMakeControllerCommand(app *hex.App) *cobra.Command {
	var (
		all     bool
		actions string
		flags   genFlags
	)

	cmd := &cobra.Command{
		Use:   "make:controller <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Generate an HTTP controller",
		Long:  helpLong("make_controller"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, modulePath, err := projectRoot()
			if err != nil {
				return err
			}

			name := args[0]
			if name == "" {
				return errors.New("controller name is empty")
			}

			selected, err := resolveActions(all, actions)
			if err != nil {
				return err
			}

			pkg := generator.GoPackageName(name)
			struct_ := generator.PascalCase(name)

			data := controllerData{
				Package:     pkg,
				Struct:      struct_,
				Constructor: "New" + struct_,
				Path:        "/" + pkg,
				Variable:    generator.CamelCase(name),
				Actions:     selected,
			}

			opts, err := flags.options()
			if err != nil {
				return err
			}

			svc, err := resolveGenerator(app)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// 1. Scaffold app/controller/<name>.go.
			actionsDone, err := svc.Run(ctx, "controller", root, data, opts)
			if err != nil {
				return err
			}

			// 2. Wire routes into app/provider/routes.go. Multi-target,
			// data-dependent wiring — not a static Blueprint Wire.
			routesFile := filepath.Join(root, "app", "provider", "routes.go")

			wireActions, err := wireControllerRoutes(svc, routesFile, modulePath, data, opts)
			actionsDone = append(actionsDone, wireActions...)

			if err != nil {
				return err
			}

			return report(cmd.OutOrStdout(), actionsDone, opts, flags.format)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "scaffold full RESTful CRUD (index/show/store/update/destroy)")
	cmd.Flags().StringVar(&actions, "actions", "", "comma-separated list of actions (index,show,store,update,destroy)")
	setExample(cmd, "make_controller")
	addGeneratorFlags(cmd, &flags)

	return cmd
}

// resolveActions returns the ordered list of controllerAction to scaffold
// given the CLI flags. Precedence: --all wins over --actions; both empty
// means "just index".
func resolveActions(all bool, actions string) ([]controllerAction, error) {
	if all {
		out := make([]controllerAction, 0, len(allControllerActions))
		for _, a := range allControllerActions {
			out = append(out, a.Action)
		}

		return out, nil
	}

	if actions == "" {
		return []controllerAction{allControllerActions[0].Action}, nil
	}

	known := map[string]controllerAction{}
	for _, a := range allControllerActions {
		known[a.Name] = a.Action
	}

	requested := strings.Split(actions, ",")
	out := make([]controllerAction, 0, len(requested))
	seen := map[string]bool{}

	for _, raw := range requested {
		name := strings.TrimSpace(strings.ToLower(raw))
		if name == "" {
			continue
		}

		if seen[name] {
			continue
		}

		a, ok := known[name]
		if !ok {
			return nil, fmt.Errorf("unknown action %q (want one of: %s)", name, actionNames())
		}

		out = append(out, a)
		seen[name] = true
	}

	if len(out) == 0 {
		return nil, errors.New("--actions is empty")
	}

	// Emit in canonical order (index, show, store, update, destroy).
	sort.SliceStable(out, func(i, j int) bool {
		return actionRank(out[i].Method) < actionRank(out[j].Method)
	})

	return out, nil
}

func actionNames() string {
	names := make([]string, 0, len(allControllerActions))
	for _, a := range allControllerActions {
		names = append(names, a.Name)
	}

	return strings.Join(names, ", ")
}

func actionRank(method string) int {
	for i, a := range allControllerActions {
		if a.Action.Method == method {
			return i
		}
	}

	return len(allControllerActions)
}

// wireControllerRoutes converts the controller/<pkg> blank import in
// routes.go into a non-blank one (if needed), then inserts the
// route-registration lines above the hex:routes marker.
func wireControllerRoutes(svc *generator.Service, routesFile, modulePath string, data controllerData, opts generator.Options) ([]generator.Action, error) {
	var actions []generator.Action

	// Promote blank controller import to a real one, once. Idempotent:
	// no-op after the first controller is scaffolded.
	act, err := svc.PromoteImport(routesFile, modulePath+"/app/controller", opts)
	if err != nil {
		return actions, err
	}

	if act != nil {
		actions = append(actions, *act)
	}

	var b bytes.Buffer

	fmt.Fprintf(&b, "%s := controller.%s()\n", data.Variable, data.Constructor)
	for _, a := range data.Actions {
		fmt.Fprintf(&b, "\te.%s(\"%s%s\", %s.%s)\n",
			methodCall(a.Verb), data.Path, a.Suffix, data.Variable, a.Method)
	}

	wireAct, err := svc.WireMarker(routesFile, "// hex:routes", b.String(), "added "+data.Struct, opts)
	if err != nil {
		return actions, fmt.Errorf("wire routes into %s: %w", routesFile, err)
	}

	if wireAct != nil {
		actions = append(actions, *wireAct)
	}

	return actions, nil
}

// methodCall maps an HTTP verb to echo.Echo's method name. GET → "GET",
// POST → "POST", etc. — they happen to match one-for-one, but keeping
// this helper isolates any future divergence.
func methodCall(verb string) string { return strings.ToUpper(verb) }
