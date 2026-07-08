package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

// providerData feeds the provider template.
type providerData struct {
	Name       string // pascalCase (Payments)
	Package    string // domain/module name for imports; unused here
	ModulePath string
}

func newMakeProviderCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "make:provider <name>",
		Short: "Generate a service provider",
		Long: "Create provider/<name>.go and wire it into provider/boot.go.\n\n" +
			"The name is normalised to PascalCase for the type and lower-case for the filename.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, modulePath, err := projectRoot()
			if err != nil {
				return err
			}

			name := args[0]
			if name == "" {
				return errors.New("provider name is empty")
			}

			data := providerData{
				Name:       pascalCase(name),
				ModulePath: modulePath,
			}

			target := filepath.Join(root, "provider", snakeCase(name)+".go")

			g := newGenerator()
			g.force = force

			if err := g.render("templates/provider.go.tmpl", target, data); err != nil {
				return err
			}

			// Wire into boot.go.
			bootFile := filepath.Join(root, "provider", "boot.go")
			registration := fmt.Sprintf("&%s{},", data.Name)

			if err := insertBeforeMarker(bootFile, "// hex:providers", registration); err != nil {
				return fmt.Errorf("wire into boot.go: %w", err)
			}

			fmt.Println("→", bootFile, "(added", data.Name+")")

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")

	return cmd
}
