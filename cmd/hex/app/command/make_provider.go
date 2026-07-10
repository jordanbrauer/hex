package command

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

// providerData feeds the provider blueprint.
type providerData struct {
	Name       string // pascalCase (Payments)
	Package    string // snake_case file/import name (payments)
	ModulePath string
}

func newMakeProviderCommand(app *hex.App) *cobra.Command {
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
				Name:       generator.PascalCase(name),
				Package:    generator.SnakeCase(name),
				ModulePath: modulePath,
			}

			opts, err := flags.options()
			if err != nil {
				return err
			}

			svc, err := resolveGenerator(app)
			if err != nil {
				return err
			}

			actions, err := svc.Run(cmd.Context(), "provider", root, data, opts)
			if err != nil {
				return err
			}

			return report(cmd.OutOrStdout(), actions, opts, flags.format)
		},
	}

	setExample(cmd, "make_provider")
	addGeneratorFlags(cmd, &flags)

	return cmd
}
