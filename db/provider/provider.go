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
	"embed"
	"errors"
	"fmt"
	"io/fs"

	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/config"
	"github.com/jordanbrauer/hex/container"
	hexdb "github.com/jordanbrauer/hex/db"
	dblua "github.com/jordanbrauer/hex/db/lua"
	hexlog "github.com/jordanbrauer/hex/log"
	hexlua "github.com/jordanbrauer/hex/lua"
	"github.com/jordanbrauer/hex/provider"
)

//go:embed config
var configFS embed.FS

// Configs returns the embedded default TOML + CUE files this provider
// contributes to hex/config. Add it to hex/config.Provider.Sources.
func Configs() fs.FS {
	sub, err := fs.Sub(configFS, "config")
	if err != nil {
		panic("db/provider: embedded config subdir missing: " + err.Error())
	}

	return sub
}

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
// connection open happens in Boot so context is available. When a
// shared hex/lua.Environment is bound in the container (i.e.
// hex/lua/provider is registered), this provider also installs the
// 'db' Lua module against it so scripts and the REPL can query the
// live connection.
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

	p.installLuaModule(app, binding)

	return nil
}

// installLuaModule preloads the 'db' Lua module against the shared
// *hex/lua.Environment if one is bound in the container. Silently
// no-ops when Lua isn't wired — hex/db is usable without Lua.
//
// Resolution of the *sql.DB is deferred to the first require("db")
// so we don't poison the singleton with the pre-Boot error (pi-7ag).
func (p *Provider) installLuaModule(app provider.Application, dbBinding string) {
	env, err := container.Make[*hexlua.Environment](app, "lua")
	if err != nil || env == nil {
		return
	}

	bindings := &dblua.Bindings{}

	env.SetType("db", dblua.TypeStub)

	env.PreloadModule("db", func(L *glua.LState) int {
		if bindings.DB == nil {
			db, err := container.Make[*sql.DB](app, dbBinding)
			if err != nil {
				L.RaiseError("db/provider: resolve %q: %v", dbBinding, err)

				return 0
			}

			bindings.DB = db
		}

		return bindings.Loader(L)
	})
}

// Boot opens the connection, applies pragmas, and runs the Migrator.
func (p *Provider) Boot(ctx context.Context, app provider.Application) error {
	store, err := container.Make[*config.Store](app, "config")
	if err != nil {
		return fmt.Errorf("db/provider: resolve config: %w", err)
	}

	cfg := p.buildConfig(store)

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

// buildConfig reads the Namespace-scoped values from the resolved
// *config.Store into a hexdb.Config.
func (p *Provider) buildConfig(store *config.Store) hexdb.Config {
	ns := p.Namespace
	if ns == "" {
		ns = "database"
	}

	return hexdb.Config{
		Driver:          store.String(ns + ".driver"),
		DSN:             store.String(ns + ".dsn"),
		MaxOpenConns:    store.Int(ns + ".pool.max_open_conns"),
		MaxIdleConns:    store.Int(ns + ".pool.max_idle_conns"),
		ConnMaxLifetime: store.Duration(ns + ".pool.conn_max_lifetime"),
		ConnMaxIdleTime: store.Duration(ns + ".pool.conn_max_idle_time"),
		Pragmas:         store.StringSlice(ns + ".pragmas"),
	}
}

// Compile-time confirmation the provider participates in shutdown.
var _ provider.Shutdowner = (*Provider)(nil)
