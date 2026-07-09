package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
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

func newMakeCommandCommand() *cobra.Command {
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
				Name:       goPackageName(name),
				FuncName:   pascalCase(name),
				Group:      goPackageName(group),
				GroupFunc:  pascalCase(group),
				ModulePath: modulePath,
			}
			data.GroupPackage = data.Group

			g, err := newGeneratorFromFlags(flags)
			if err != nil {
				return err
			}

			if data.Group == "" {
				if err := makeTopLevelCommand(g, root, data); err != nil {
					return err
				}
			} else if err := makeGroupCommand(g, root, data); err != nil {
				return err
			}

			return g.report()
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "parent command group (creates a subcommand)")
	setExample(cmd, "make_command")
	addGeneratorFlags(cmd, &flags)

	return cmd
}

// makeTopLevelCommand generates app/command/<name>.go and wires it into
// app/command/root.go via the hex:commands marker.
func makeTopLevelCommand(g *generator, root string, data commandData) error {
	target := filepath.Join(root, "app", "command", data.Name+".go")
	if err := g.render("templates/command.go.tmpl", target, data); err != nil {
		return err
	}

	// Register in the top-level root.go via the hex:commands marker.
	rootFile := filepath.Join(root, "app", "command", "root.go")
	registration := fmt.Sprintf("%s(app),", data.FuncName)

	if err := g.wireMarker(rootFile, "// hex:commands", registration, "added "+data.FuncName); err != nil {
		return fmt.Errorf("wire into %s: %w", rootFile, err)
	}

	return nil
}

// makeGroupCommand generates the group root (if missing) and the
// subcommand, then wires everything up.
func makeGroupCommand(g *generator, root string, data commandData) error {
	groupDir := filepath.Join(root, "app", "command", data.Group)

	if err := g.mkdirp(groupDir); err != nil {
		return err
	}

	// Generate the group's root.go if not present. force does NOT apply
	// here — regenerating the group root would erase any subcommand
	// registrations the user has added.
	groupRoot := filepath.Join(groupDir, "root.go")

	if _, err := os.Stat(groupRoot); errors.Is(err, os.ErrNotExist) {
		if err := g.render("templates/command_group.go.tmpl", groupRoot, data); err != nil {
			return err
		}

		// New group: register the group's root in the top-level root.go.
		topRoot := filepath.Join(root, "app", "command", "root.go")
		reg := fmt.Sprintf("%s.Root(app),", data.Group)

		if err := g.wireMarker(topRoot, "// hex:commands", reg, "added "+data.Group+".Root"); err != nil {
			return fmt.Errorf("wire group into %s: %w", topRoot, err)
		}

		// Add the group's package import to root.go.
		if err := g.wireImport(topRoot, data.ModulePath+"/app/command/"+data.Group); err != nil {
			return fmt.Errorf("add import to %s: %w", topRoot, err)
		}
	}

	// Now the subcommand itself.
	target := filepath.Join(groupDir, data.Name+".go")
	if err := g.render("templates/command_sub.go.tmpl", target, data); err != nil {
		return err
	}

	// Register the subcommand in the group's root.go.
	registration := fmt.Sprintf("%s(app),", data.FuncName)

	if err := g.wireMarker(groupRoot, "// hex:commands:"+data.Group, registration, "added "+data.FuncName); err != nil {
		return fmt.Errorf("wire into %s: %w", groupRoot, err)
	}

	return nil
}
