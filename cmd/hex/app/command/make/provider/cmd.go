// Package provider implements `hex make provider`.
package provider

import (
	_ "embed"
	"errors"
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

// providerData feeds the provider blueprint.
type providerData struct {
	Name       string // pascalCase (Payments)
	Package    string // snake_case file/import name (payments)
	ModulePath string
}

// New builds the `hex make provider <name>` command.
func New(app *hex.App) *cobra.Command {
	var flags generator.Flags

	cmd := &cobra.Command{
		Use:     "provider <name>",
		Args:    cobra.ExactArgs(1),
		Short:   "Generate a service provider",
		Long:    long,
		Example: example,
		RunE:    run(app, &flags),
	}

	generator.AddFlags(cmd, &flags)

	return cmd
}

func run(app *hex.App, flags *generator.Flags) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		root, modulePath, err := generator.ProjectRoot()
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

		opts, err := flags.Options()
		if err != nil {
			return err
		}

		svc, err := generator.Resolve(app)
		if err != nil {
			return err
		}

		actions, err := svc.Run(cmd.Context(), "provider", root, data, opts)
		if err != nil {
			return err
		}

		return generator.Report(cmd.OutOrStdout(), actions, opts.DryRun, flags.Format)
	}
}
