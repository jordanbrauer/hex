// Package adapter implements `hex make adapter`.
package adapter

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

//go:embed long.md
var longFile string

//go:embed example.sh
var exampleFile string

var (
	long    = strings.TrimRight(longFile, "\n")
	example = strings.TrimRight(exampleFile, "\n")
)

// adapterData feeds the adapter blueprint.
type adapterData struct {
	Domain     string // package name of the domain ("user")
	DomainType string // exported type name ("User")
	Dialect    string // "sqlite" or "postgres"
	ModulePath string
}

// New builds the `hex make adapter <domain>` command.
func New(app *hex.App) *cobra.Command {
	var (
		dialect string
		flags   generator.Flags
	)

	cmd := &cobra.Command{
		Use:     "adapter <domain>",
		Args:    cobra.ExactArgs(1),
		Short:   "Generate an infrastructure adapter for a domain repository",
		Long:    long,
		Example: example,
		RunE:    run(app, &dialect, &flags),
	}

	cmd.Flags().StringVar(&dialect, "dialect", "sqlite", "SQL dialect: sqlite or postgres")
	generator.AddFlags(cmd, &flags)

	return cmd
}

func run(app *hex.App, dialect *string, flags *generator.Flags) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		root, modulePath, err := generator.ProjectRoot()
		if err != nil {
			return err
		}

		domain := args[0]
		if domain == "" {
			return errors.New("domain name is empty")
		}

		if *dialect != "sqlite" && *dialect != "postgres" {
			return fmt.Errorf("unsupported dialect %q (want sqlite or postgres)", *dialect)
		}

		data := adapterData{
			Domain:     generator.GoPackageName(domain),
			DomainType: generator.PascalCase(domain),
			Dialect:    *dialect,
			ModulePath: modulePath,
		}

		opts, err := flags.Options()
		if err != nil {
			return err
		}

		svc, err := generator.Resolve(app)
		if err != nil {
			return err
		}

		actions, err := svc.Run(cmd.Context(), "adapter", root, data, opts)
		if err != nil {
			return err
		}

		return generator.Report(cmd.OutOrStdout(), actions, opts.DryRun, flags.Format)
	}
}
