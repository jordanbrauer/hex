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
		force bool
	)

	cmd := &cobra.Command{
		Use:   "make:command <name>",
		Short: "Generate a cobra command",
		Long: `Create a new cobra command wired into the application.

Without --group, the command lands at app/command/<name>.go and is
registered against the root's hex:commands marker.

With --group, the command lands at app/command/<group>/<name>.go and
is registered against the group's hex:commands:<group> marker. The
group's root.go is generated automatically if it does not exist yet.`,
		Args: cobra.ExactArgs(1),
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

			g := newGenerator()
			g.force = force

			if data.Group == "" {
				return makeTopLevelCommand(g, root, data)
			}

			return makeGroupCommand(g, root, data)
		},
	}

	cmd.Flags().StringVar(&group, "group", "", "parent command group (creates a subcommand)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")

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

	if err := insertBeforeMarker(rootFile, "// hex:commands", registration); err != nil {
		return fmt.Errorf("wire into %s: %w", rootFile, err)
	}

	fmt.Println("→", rootFile, "(added", data.FuncName+")")

	return nil
}

// makeGroupCommand generates the group root (if missing) and the
// subcommand, then wires everything up.
func makeGroupCommand(g *generator, root string, data commandData) error {
	groupDir := filepath.Join(root, "app", "command", data.Group)

	if err := os.MkdirAll(groupDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", groupDir, err)
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

		if err := insertBeforeMarker(topRoot, "// hex:commands", reg); err != nil {
			return fmt.Errorf("wire group into %s: %w", topRoot, err)
		}

		// Add the group's package import to root.go. Import is inserted
		// above the closing `)` of the import block via a second marker
		// convention: no marker required because Go's imports are stable
		// enough to detect by string match. We use a simple approach:
		// if the import is not present, add it above the last `)`.
		if err := addImport(topRoot, data.ModulePath+"/app/command/"+data.Group); err != nil {
			return fmt.Errorf("add import to %s: %w", topRoot, err)
		}

		fmt.Println("→", topRoot, "(added", data.Group+".Root)")
	}

	// Now the subcommand itself.
	target := filepath.Join(groupDir, data.Name+".go")
	if err := g.render("templates/command_sub.go.tmpl", target, data); err != nil {
		return err
	}

	// Register the subcommand in the group's root.go.
	registration := fmt.Sprintf("%s(app),", data.FuncName)

	if err := insertBeforeMarker(groupRoot, "// hex:commands:"+data.Group, registration); err != nil {
		return fmt.Errorf("wire into %s: %w", groupRoot, err)
	}

	fmt.Println("→", groupRoot, "(added", data.FuncName+")")

	return nil
}
