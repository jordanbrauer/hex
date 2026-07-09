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
	var flags genFlags

	cmd := &cobra.Command{
		Use:   "make:provider <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Generate a service provider",
		Long:  helpLong("make_provider"),
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

			target := filepath.Join(root, "app", "provider", snakeCase(name)+".go")

			g, err := newGeneratorFromFlags(flags)
			if err != nil {
				return err
			}

			if err := g.render("templates/provider.go.tmpl", target, data); err != nil {
				return err
			}

			// Wire into app/boot.go.
			bootFile := filepath.Join(root, "app", "boot.go")
			registration := fmt.Sprintf("&provider.%s{},", data.Name)

			if err := g.wireMarker(bootFile, "// hex:providers", registration, "added "+data.Name); err != nil {
				return fmt.Errorf("wire into boot.go: %w", err)
			}

			return g.report()
		},
	}

	setExample(cmd, "make_provider")
	addGeneratorFlags(cmd, &flags)

	return cmd
}
