// Package postgres runs golang-migrate migrations against a Postgres database.
//
// This package intentionally lives outside hex/db so consumers who use a
// different dialect never link the Postgres migration driver. Import it
// only when you need Postgres migrations.
//
// The SQL driver itself (e.g. github.com/lib/pq or github.com/jackc/pgx)
// is still the caller's responsibility to blank-import.
package postgres

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	migpg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrate applies all pending Up migrations from the embedded FS. Returns nil
// if there are no changes.
//
// The caller retains ownership of sqldb; this function never closes it (even
// though golang-migrate's Migrate.Close would). The iofs source has no
// external resources to release, so no cleanup is required.
func Migrate(sqldb *sql.DB, migrations embed.FS, dir string) error {
	m, err := newMigrator(sqldb, migrations, dir)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("postgres: migrate up: %w", err)
	}

	return nil
}

// MigrateDown rolls back every applied migration. sqldb is not closed.
func MigrateDown(sqldb *sql.DB, migrations embed.FS, dir string) error {
	m, err := newMigrator(sqldb, migrations, dir)
	if err != nil {
		return err
	}

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("postgres: migrate down: %w", err)
	}

	return nil
}

// MigrateSteps applies n migrations up (n > 0) or down (n < 0). sqldb is not
// closed.
func MigrateSteps(sqldb *sql.DB, migrations embed.FS, dir string, n int) error {
	m, err := newMigrator(sqldb, migrations, dir)
	if err != nil {
		return err
	}

	if err := m.Steps(n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("postgres: migrate steps %d: %w", n, err)
	}

	return nil
}

func newMigrator(sqldb *sql.DB, migrations embed.FS, dir string) (*migrate.Migrate, error) {
	if dir == "" {
		dir = "."
	}

	if _, err := fs.ReadDir(migrations, dir); err != nil {
		return nil, fmt.Errorf("postgres: open embedded migrations dir %q: %w", dir, err)
	}

	source, err := iofs.New(migrations, dir)
	if err != nil {
		return nil, fmt.Errorf("postgres: open embedded migrations: %w", err)
	}

	driver, err := migpg.WithInstance(sqldb, &migpg.Config{})
	if err != nil {
		return nil, fmt.Errorf("postgres: init migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("postgres: init migrator: %w", err)
	}

	return m, nil
}
