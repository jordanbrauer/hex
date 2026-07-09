package provider

import (
	"database/sql"

	_ "modernc.org/sqlite"

	"github.com/jordanbrauer/hex/db/provider"
	"github.com/jordanbrauer/hex/db/sqlite"

	"github.com/jordanbrauer/hex/examples/swapi/database"
)

// Database wires the app's primary SQL connection. All the plumbing
// (config reading, connection open, pool tuning, migration runner)
// lives in hex/db/provider — modify by adding hooks below, or by
// replacing this factory entirely.
func Database() *provider.Provider {
	return &provider.Provider{
		Namespace: "database",
		Binding:   "db",
		Migrator: func(db *sql.DB) error {
			return sqlite.Migrate(db, database.Migrations, database.MigrationsDir)
		},

		// BeforeOpen: func(ctx context.Context, cfg hexdb.Config) hexdb.Config {
		//     cfg.Pragmas = append(cfg.Pragmas, "PRAGMA foreign_keys = ON")
		//     return cfg
		// },
		//
		// AfterOpen: func(ctx context.Context, db *sql.DB) error {
		//     // e.g. warm a cache, register listeners
		//     return nil
		// },
	}
}
