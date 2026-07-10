package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

// commandData feeds the command templates.
type commandData struct {
	// Name is the subcommand name (e.g. "login" for `myapp auth login`).
	Name string
	// FuncName is the exported function that returns the *cobra.Command
	// (e.g. "Login" for `command.Login(app)`).
	FuncName string
	// Group is the parent command group (e.g. "auth"). Empty means the
	// command sits directly under the root.
	Group string
	// GroupFunc is the group's root function ("Auth" for `command.Auth(app)`).
	GroupFunc string
	// ModulePath is this project's go.mod module path.
	ModulePath string
	// GroupPackage is the group's Go package name (lower-cased Group).
	GroupPackage string
}

func newMakeCommandCommand(app *hex.App) *cobra.Command {
	var (
		group string
		flags genFlags
	)

	cmd := &cobra.Command{
		Use:   "make:command <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Generate a cobra command",
		Long:  helpLong("make_command"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, modulePath, err := projectRoot()
			if err != nil {
				return err
			}

			name := args[0]
			if name == "" {
				return errors.New("command name is empty")
			}

			data := commandData{
				Name:       generator.GoPackageName(name),
				FuncName:   generator.PascalCase(name),
				Group:      generator.GoPackageName(group),
				GroupFunc:  generator.PascalCase(group),
				ModulePath: modulePath,
			}
			data.GroupPackage = data.Group

			opts, err := flags.options()
			if err != nil {
				return err
			}

			svc, err := resolveGenerator(app)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			var actions []generator.Action

			if data.Group == "" {
				actions, err = svc.Run(ctx, "command", root, data, opts)
			} else {
				actions, err = makeGroupCommand(ctx, svc, root, data, opts)
			}

			if err != nil {
				return err
			}

			return report(cmd.OutOrStdout(), actions, opts, flags.format)
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "parent command group (creates a subcommand)")
	setExample(cmd, "make_command")
	addGeneratorFlags(cmd, &flags)

	return cmd
}

// makeGroupCommand generates the group root (if missing) and the
// subcommand, then wires everything up. This is conditional,
// multi-target wiring — not a static Blueprint — so it calls Service's
// primitive methods directly instead of Service.Run.
func makeGroupCommand(ctx context.Context, svc *generator.Service, root string, data commandData, opts generator.Options) ([]generator.Action, error) {
	var actions []generator.Action

	groupDir := filepath.Join(root, "app", "command", data.Group)

	if act, err := svc.Mkdirp(groupDir, opts); err != nil {
		return actions, err
	} else if act != nil {
		actions = append(actions, *act)
	}

	// Generate the group's root.go if not present. force does NOT apply
	// here — regenerating the group root would erase any subcommand
	// registrations the user has added.
	groupRoot := filepath.Join(groupDir, "root.go")

	if _, err := os.Stat(groupRoot); errors.Is(err, os.ErrNotExist) {
		act, err := svc.RenderFile(ctx, "templates/command_group.go.tmpl", groupRoot, data, opts)
		if err != nil {
			return actions, err
		}

		actions = append(actions, act)

		// New group: register the group's root in the top-level root.go.
		topRoot := filepath.Join(root, "app", "command", "root.go")
		reg := fmt.Sprintf("%s.Root(app),", data.Group)

		wireAct, err := svc.WireMarker(topRoot, "// hex:commands", reg, "added "+data.Group+".Root", opts)
		if err != nil {
			return actions, fmt.Errorf("wire group into %s: %w", topRoot, err)
		}

		if wireAct != nil {
			actions = append(actions, *wireAct)
		}

		// Add the group's package import to root.go.
		importAct, err := svc.WireImport(topRoot, data.ModulePath+"/app/command/"+data.Group, opts)
		if err != nil {
			return actions, fmt.Errorf("add import to %s: %w", topRoot, err)
		}

		if importAct != nil {
			actions = append(actions, *importAct)
		}
	}

	// Now the subcommand itself.
	target := filepath.Join(groupDir, data.Name+".go")

	act, err := svc.RenderFile(ctx, "templates/command_sub.go.tmpl", target, data, opts)
	if err != nil {
		return actions, err
	}

	actions = append(actions, act)

	// Register the subcommand in the group's root.go.
	registration := fmt.Sprintf("%s(app),", data.FuncName)

	wireAct, err := svc.WireMarker(groupRoot, "// hex:commands:"+data.Group, registration, "added "+data.FuncName, opts)
	if err != nil {
		return actions, fmt.Errorf("wire into %s: %w", groupRoot, err)
	}

	if wireAct != nil {
		actions = append(actions, *wireAct)
	}

	return actions, nil
}
