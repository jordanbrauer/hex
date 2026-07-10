package command

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

// domainData feeds the domain blueprint's four files.
type domainData struct {
	Package    string // "user"
	Name       string // "User"
	Plural     string // "Users"
	ModulePath string
}

func newMakeDomainCommand(app *hex.App) *cobra.Command {
	var flags genFlags

	cmd := &cobra.Command{
		Use:   "make:domain <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Generate a domain package",
		Long:  helpLong("make_domain"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, modulePath, err := projectRoot()
			if err != nil {
				return err
			}

			name := args[0]
			if name == "" {
				return errors.New("domain name is empty")
			}

			data := domainData{
				Package:    generator.GoPackageName(name),
				Name:       generator.PascalCase(name),
				Plural:     generator.Pluralise(generator.PascalCase(name)),
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

			actions, err := svc.Run(cmd.Context(), "domain", root, data, opts)
			if err != nil {
				return err
			}

			return report(cmd.OutOrStdout(), actions, opts, flags.format)
		},
	}

	setExample(cmd, "make_domain")
	addGeneratorFlags(cmd, &flags)

	return cmd
}
