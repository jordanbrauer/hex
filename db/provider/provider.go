// Package provider is the default hex/db service provider.
//
// It reads driver + DSN + pool tuning from hex/config, opens the
// database, applies pragmas, and binds *sql.DB into the container
// under a caller-configurable name (default "db"). Optionally runs
// embedded migrations if the caller passes a Migrator.
//
// Hooks (BeforeOpen / AfterOpen / BeforeMigrate) let consumers reach
// in for customisations without replacing the whole provider.
package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jordanbrauer/hex/config"
	"github.com/jordanbrauer/hex/container"
	hexdb "github.com/jordanbrauer/hex/db"
	hexlog "github.com/jordanbrauer/hex/log"
	"github.com/jordanbrauer/hex/provider"
)

// Migrator runs schema migrations against sqldb. hex/db/sqlite and
// hex/db/postgres both expose a Migrate function that satisfies this
// shape via a small closure: `func(db *sql.DB) error { return
// sqlite.Migrate(db, fs, dir) }`.
type Migrator func(sqldb *sql.DB) error

// Provider wires *sql.DB into the container.
type Provider struct {
	provider.Base

	// Binding is the container name under which *sql.DB is bound.
	// Defaults to "db".
	Binding string

	// Namespace is the config namespace read for db settings (driver,
	// dsn, pool, pragmas). Defaults to "database".
	Namespace string

	// Migrator, when non-nil, runs after the connection is open and
	// pinged. Zero means "no migrations."
	Migrator Migrator

	// Hooks.
	BeforeOpen    func(ctx context.Context, cfg hexdb.Config) hexdb.Config
	AfterOpen     func(ctx context.Context, db *sql.DB) error
	BeforeMigrate func(ctx context.Context, db *sql.DB) error

	db *sql.DB
}

// Register binds the *sql.DB into the container lazily — the actual
// connection open happens in Boot so context is available.
func (p *Provider) Register(app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "db"
	}

	app.Singleton(binding, func(*container.Container) (any, error) {
		if p.db == nil {
			return nil, errors.New("db/provider: connection not yet opened (Boot has not run)")
		}

		return p.db, nil
	})

	return nil
}

// Boot opens the connection, applies pragmas, and runs the Migrator.
func (p *Provider) Boot(ctx context.Context, app provider.Application) error {
	cfg := p.buildConfig()

	if p.BeforeOpen != nil {
		cfg = p.BeforeOpen(ctx, cfg)
	}

	hexlog.Debug("db/provider: opening", "driver", cfg.Driver)

	sqldb, err := hexdb.Open(ctx, cfg)
	if err != nil {
		return fmt.Errorf("db/provider: open: %w", err)
	}

	if p.AfterOpen != nil {
		if err := p.AfterOpen(ctx, sqldb); err != nil {
			_ = sqldb.Close()

			return fmt.Errorf("db/provider: AfterOpen: %w", err)
		}
	}

	if p.Migrator != nil {
		if p.BeforeMigrate != nil {
			if err := p.BeforeMigrate(ctx, sqldb); err != nil {
				_ = sqldb.Close()

				return fmt.Errorf("db/provider: BeforeMigrate: %w", err)
			}
		}

		if err := p.Migrator(sqldb); err != nil {
			_ = sqldb.Close()

			return fmt.Errorf("db/provider: migrate: %w", err)
		}
	}

	p.db = sqldb

	hexlog.Info("db/provider: ready", "driver", cfg.Driver)

	return nil
}

// Shutdown closes the connection.
func (p *Provider) Shutdown(ctx context.Context, app provider.Application) error {
	if p.db == nil {
		return nil
	}

	err := p.db.Close()
	p.db = nil

	return err
}

// buildConfig reads the Namespace-scoped values from hex/config into a
// hexdb.Config.
func (p *Provider) buildConfig() hexdb.Config {
	ns := p.Namespace
	if ns == "" {
		ns = "database"
	}

	cfg := hexdb.Config{
		Driver:          config.String(ns + ".driver"),
		DSN:             config.String(ns + ".dsn"),
		MaxOpenConns:    config.Int(ns + ".pool.max_open_conns"),
		MaxIdleConns:    config.Int(ns + ".pool.max_idle_conns"),
		ConnMaxLifetime: config.Duration(ns + ".pool.conn_max_lifetime"),
		ConnMaxIdleTime: config.Duration(ns + ".pool.conn_max_idle_time"),
		Pragmas:         config.StringSlice(ns + ".pragmas"),
	}

	// Ensure the driver/DSN error paths run through hex/db.Open so we
	// get consistent error messages.
	_ = time.Second // silence unused import when hooks are all nil

	return cfg
}

// Compile-time confirmation the provider participates in shutdown.
var _ provider.Shutdowner = (*Provider)(nil)
