// Package db provides driver-agnostic helpers for opening *sql.DB
// connections and tuning them for hex applications.
//
// hex/db imports no SQL driver and no migration tooling. Consumers:
//
//   - Blank-import their driver of choice
//     (e.g. modernc.org/sqlite, github.com/lib/pq).
//   - Import a companion migration subpackage
//     (hex/db/sqlite, hex/db/postgres) if they use golang-migrate.
//
// This keeps hex/db's dependency footprint tiny and prevents unused drivers
// from being linked into every binary. See ADR-0004.
//
// Example:
//
//	import (
//	    _ "modernc.org/sqlite"
//	    "github.com/jordanbrauer/hex/db"
//	    hexsqlite "github.com/jordanbrauer/hex/db/sqlite"
//	)
//
//	//go:embed migrations/*.sql
//	var migrations embed.FS
//
//	conn, err := db.Open(ctx, db.Config{
//	    Driver:  "sqlite",
//	    DSN:     "/var/lib/myapp.db",
//	    Pragmas: []string{"journal_mode = WAL"},
//	})
//	if err != nil { return err }
//	if err := hexsqlite.Migrate(conn, migrations, "migrations"); err != nil {
//	    return err
//	}
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Config describes how to open a database connection.
type Config struct {
	// Driver is the sql.Open driver name (e.g. "sqlite", "postgres"). The
	// consumer must blank-import the driver package for this name to
	// resolve at runtime.
	Driver string

	// DSN is the data source name passed to sql.Open. Format is
	// driver-specific.
	DSN string

	// Pragmas are executed once immediately after the connection is opened.
	// The strings are passed through to Exec verbatim, so callers control
	// whether to prefix "PRAGMA ", "SET ", etc.
	Pragmas []string

	// Pool configuration. Zero means "use driver default".
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// Open opens a connection, applies pragmas and pool tuning, and verifies the
// connection with PingContext. The returned *sql.DB is the caller's to
// close.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	if cfg.Driver == "" {
		return nil, errors.New("db: Config.Driver is required")
	}

	if cfg.DSN == "" {
		return nil, errors.New("db: Config.DSN is required")
	}

	sqldb, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", cfg.Driver, err)
	}

	if err := setup(ctx, sqldb, cfg); err != nil {
		_ = sqldb.Close()

		return nil, err
	}

	return sqldb, nil
}

// Tune applies pragmas and pool settings to an already-opened connection.
// Useful when a consumer opens *sql.DB themselves (for a special driver
// setup) but still wants hex's uniform tuning.
func Tune(ctx context.Context, sqldb *sql.DB, cfg Config) error {
	return setup(ctx, sqldb, cfg)
}

func setup(ctx context.Context, sqldb *sql.DB, cfg Config) error {
	if cfg.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(cfg.MaxOpenConns)
	}

	if cfg.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	if cfg.ConnMaxLifetime > 0 {
		sqldb.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	if cfg.ConnMaxIdleTime > 0 {
		sqldb.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	for _, stmt := range cfg.Pragmas {
		if _, err := sqldb.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("db: exec %q: %w", stmt, err)
		}
	}

	if err := sqldb.PingContext(ctx); err != nil {
		return fmt.Errorf("db: ping: %w", err)
	}

	return nil
}
