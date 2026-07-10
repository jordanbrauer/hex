package command

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

// adapterData feeds the adapter blueprint.
type adapterData struct {
	Domain     string // package name of the domain ("user")
	DomainType string // exported type name ("User")
	Dialect    string // "sqlite" or "postgres"
	ModulePath string
}

func newMakeAdapterCommand(app *hex.App) *cobra.Command {
	var (
		dialect string
		flags   genFlags
	)

	cmd := &cobra.Command{
		Use:   "make:adapter <domain>",
		Args:  cobra.ExactArgs(1),
		Short: "Generate an infrastructure adapter for a domain repository",
		Long:  helpLong("make_adapter"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, modulePath, err := projectRoot()
			if err != nil {
				return err
			}

			domain := args[0]
			if domain == "" {
				return errors.New("domain name is empty")
			}

			if dialect != "sqlite" && dialect != "postgres" {
				return fmt.Errorf("unsupported dialect %q (want sqlite or postgres)", dialect)
			}

			data := adapterData{
				Domain:     generator.GoPackageName(domain),
				DomainType: generator.PascalCase(domain),
				Dialect:    dialect,
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

			actions, err := svc.Run(cmd.Context(), "adapter", root, data, opts)
			if err != nil {
				return err
			}

			return report(cmd.OutOrStdout(), actions, opts, flags.format)
		},
	}

	cmd.Flags().StringVar(&dialect, "dialect", "sqlite", "SQL dialect: sqlite or postgres")
	setExample(cmd, "make_adapter")
	addGeneratorFlags(cmd, &flags)

	return cmd
}
