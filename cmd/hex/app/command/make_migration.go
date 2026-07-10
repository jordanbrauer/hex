package command

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
)

// migrationData is available inside migration templates via
// {{.Name}} / {{.Table}} / {{.Timestamp}}.
type migrationData struct {
	Name      string // "create_users_table"
	Table     string // best-guess extracted table name ("users")
	Timestamp string // "20260708120000"
}

func newMakeMigrationCommand(app *hex.App) *cobra.Command {
	var flags genFlags

	cmd := &cobra.Command{
		Use:   "make:migration <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Generate a timestamped SQL migration",
		Long:  helpLong("make_migration"),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _, err := projectRoot()
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

			opts, err := flags.options()
			if err != nil {
				return err
			}

			svc, err := resolveGenerator(app)
			if err != nil {
				return err
			}

			actions, err := svc.Run(cmd.Context(), "migration", root, data, opts)
			if err != nil {
				return err
			}

			return report(cmd.OutOrStdout(), actions, opts, flags.format)
		},
	}

	setExample(cmd, "make_migration")
	addGeneratorFlags(cmd, &flags)

	return cmd
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
