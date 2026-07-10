// Package domain implements `hex make domain`.
package domain

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

// domainData feeds the domain blueprint's four files.
type domainData struct {
	Package    string // "user"
	Name       string // "User"
	Plural     string // "Users"
	ModulePath string
}

// New builds the `hex make domain <name>` command.
func New(app *hex.App) *cobra.Command {
	var flags generator.Flags

	cmd := &cobra.Command{
		Use:     "domain <name>",
		Args:    cobra.ExactArgs(1),
		Short:   "Generate a domain package",
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
			return errors.New("domain name is empty")
		}

		data := domainData{
			Package:    generator.GoPackageName(name),
			Name:       generator.PascalCase(name),
			Plural:     generator.Pluralise(generator.PascalCase(name)),
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

		actions, err := svc.Run(cmd.Context(), "domain", root, data, opts)
		if err != nil {
			return err
		}

		return generator.Report(cmd.OutOrStdout(), actions, opts.DryRun, flags.Format)
	}
}
