// Package migration implements `hex make migration`.
package migration

import (
	_ "embed"
	"errors"
	"strings"
	"time"

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

// migrationData is available inside migration templates via
// {{.Name}} / {{.Table}} / {{.Timestamp}}.
type migrationData struct {
	Name      string // "create_users_table"
	Table     string // best-guess extracted table name ("users")
	Timestamp string // "20260708120000"
}

// New builds the `hex make migration <name>` command.
func New(app *hex.App) *cobra.Command {
	var flags generator.Flags

	cmd := &cobra.Command{
		Use:     "migration <name>",
		Args:    cobra.ExactArgs(1),
		Short:   "Generate a timestamped SQL migration",
		Long:    long,
		Example: example,
		RunE:    run(app, &flags),
	}

	generator.AddFlags(cmd, &flags)

	return cmd
}

func run(app *hex.App, flags *generator.Flags) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		root, _, err := generator.ProjectRoot()
		if err != nil {
			return err
		}

		name := generator.SnakeCase(args[0])
		if name == "" {
			return errors.New("migration name is empty")
		}

		data := migrationData{
			Name:      name,
			Table:     guessTableName(name),
			Timestamp: time.Now().UTC().Format("20060102150405"),
		}

		opts, err := flags.Options()
		if err != nil {
			return err
		}

		svc, err := generator.Resolve(app)
		if err != nil {
			return err
		}

		actions, err := svc.Run(cmd.Context(), "migration", root, data, opts)
		if err != nil {
			return err
		}

		return generator.Report(cmd.OutOrStdout(), actions, opts.DryRun, flags.Format)
	}
}

// guessTableName pulls the table name out of a conventional migration
// filename ("create_users_table" → "users"). Falls back to the full name
// if the "create_<x>_table" pattern is not present.
func guessTableName(name string) string {
	const prefix = "create_"
	const suffix = "_table"

	if len(name) > len(prefix)+len(suffix) &&
		name[:len(prefix)] == prefix &&
		name[len(name)-len(suffix):] == suffix {
		return name[len(prefix) : len(name)-len(suffix)]
	}

	return name
}
