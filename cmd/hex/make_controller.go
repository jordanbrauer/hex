package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
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

func newMakeControllerCommand() *cobra.Command {
	var (
		all     bool
		actions string
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "make:controller <name>",
		Short: "Generate an HTTP controller",
		Long: `Create an app/controller/<name>.go controller and wire routes into
app/provider/routes.go before the ` + "`// hex:routes`" + ` marker.

Default: a single Index handler with a GET /<name> route. Use --all
to scaffold full RESTful CRUD (index/show/store/update/destroy) or
--actions to pick a subset (comma-separated).

Requires --web to have been enabled at hex init so the Routes
provider and app/controller/ package exist.`,
		Args: cobra.ExactArgs(1),
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

			pkg := goPackageName(name)  // "users"
			struct_ := pascalCase(name) // "Users"

			data := controllerData{
				Struct:      struct_,
				Constructor: "New" + struct_,
				Path:        "/" + pkg,
				Variable:    camelCase(name),
				Actions:     selected,
			}

			// 1. Scaffold app/controller/<name>.go.
			target := filepath.Join(root, "app", "controller", pkg+".go")

			g := newGenerator()
			g.force = force

			if err := g.render("templates/controller.go.tmpl", target, data); err != nil {
				return err
			}

			// 2. Wire routes into app/provider/routes.go.
			routesFile := filepath.Join(root, "app", "provider", "routes.go")
			if err := wireControllerRoutes(routesFile, modulePath, data); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "scaffold full RESTful CRUD (index/show/store/update/destroy)")
	cmd.Flags().StringVar(&actions, "actions", "", "comma-separated list of actions (index,show,store,update,destroy)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")

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
func wireControllerRoutes(routesFile, modulePath string, data controllerData) error {
	// Promote blank controller import to a real one, once. Idempotent:
	// no-op after the first controller is scaffolded.
	if err := promoteBlankImport(routesFile, modulePath+"/app/controller"); err != nil {
		return err
	}

	var b bytes.Buffer

	fmt.Fprintf(&b, "%s := controller.%s()\n", data.Variable, data.Constructor)
	for _, a := range data.Actions {
		fmt.Fprintf(&b, "\te.%s(\"%s%s\", %s.%s)\n",
			methodCall(a.Verb), data.Path, a.Suffix, data.Variable, a.Method)
	}

	if err := insertBeforeMarker(routesFile, "// hex:routes", b.String()); err != nil {
		return fmt.Errorf("wire routes into %s: %w", routesFile, err)
	}

	fmt.Println("→", routesFile, "(added", data.Struct+")")

	return nil
}

// methodCall maps an HTTP verb to echo.Echo's method name. GET → "GET",
// POST → "POST", etc. — they happen to match one-for-one, but keeping
// this helper isolates any future divergence.
func methodCall(verb string) string { return strings.ToUpper(verb) }

// promoteBlankImport rewrites the underscore-blank form of an import
// into a normal import so name references compile. Idempotent.
func promoteBlankImport(file, importPath string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}

	blank := fmt.Sprintf(`_ %q`, importPath)
	real_ := fmt.Sprintf(`%q`, importPath)

	if !bytes.Contains(data, []byte(blank)) {
		return nil // already promoted or never blank
	}

	out := bytes.ReplaceAll(data, []byte(blank), []byte(real_))

	return os.WriteFile(file, out, 0o644)
}
