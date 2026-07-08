package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// migrationData is available inside migration templates via
// {{.Name}} / {{.Table}} / {{.Timestamp}}.
type migrationData struct {
	Name      string // "create_users_table"
	Table     string // best-guess extracted table name ("users")
	Timestamp string // "20260708120000"
}

func newMakeMigrationCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "make:migration <name>",
		Short: "Generate a timestamped SQL migration",
		Long: "Create db/migrations/<timestamp>_<name>.{up,down}.sql\n\n" +
			"The timestamp is in the format golang-migrate expects (yyyyMMddHHmmss),\n" +
			"lexically sortable and unique to the second. Consumers can then edit the\n" +
			"generated SQL to add their real schema.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, _, err := projectRoot()
			if err != nil {
				return err
			}

			name := snakeCase(args[0])
			if name == "" {
				return errors.New("migration name is empty")
			}

			data := migrationData{
				Name:      name,
				Table:     guessTableName(name),
				Timestamp: time.Now().UTC().Format("20060102150405"),
			}

			base := fmt.Sprintf("%s_%s", data.Timestamp, data.Name)
			dir := filepath.Join(root, "db", "migrations")

			files := []struct{ tpl, target string }{
				{"templates/migration.up.sql.tmpl", filepath.Join(dir, base+".up.sql")},
				{"templates/migration.down.sql.tmpl", filepath.Join(dir, base+".down.sql")},
			}

			g := newGenerator()
			g.force = force

			for _, f := range files {
				if err := g.render(f.tpl, f.target, data); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")

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
