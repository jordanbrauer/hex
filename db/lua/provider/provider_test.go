package provider_test

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/container"
	dbluaprovider "github.com/jordanbrauer/hex/db/lua/provider"
	"github.com/jordanbrauer/hex/env"
	hexlua "github.com/jordanbrauer/hex/lua"
	luaprovider "github.com/jordanbrauer/hex/lua/provider"
	hexprovider "github.com/jordanbrauer/hex/provider"
)

// eagerDBProvider binds an already-open *sql.DB. Sidesteps the full
// hex/db/provider which reads config + opens a real connection —
// this test just wants to prove the wiring between hex/lua/provider,
// hex/db/lua/provider, and a container-bound DB.
type eagerDBProvider struct {
	hexprovider.Base
	db *sql.DB
}

func (p *eagerDBProvider) Register(app hexprovider.Application) error {
	app.Singleton("db", func(*container.Container) (any, error) {
		return p.db, nil
	})

	return nil
}

func TestProvider_installsDBModuleThroughContainer(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if _, err := conn.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := conn.Exec(`INSERT INTO users (name) VALUES ('alice')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	app := hex.New(hex.WithEnvironment(env.Test))

	err = app.Register(
		&luaprovider.Provider{},
		&eagerDBProvider{db: conn},
		&dbluaprovider.Provider{},
	)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	luaEnv, err := container.Make[*hexlua.Environment](app.Container(), "lua")
	if err != nil {
		t.Fatalf("resolve lua: %v", err)
	}

	err = luaEnv.ExecString(`
		local db = require("db")
		local u, err = db.queryOne("SELECT id, name FROM users WHERE name = ?", "alice")
		if err ~= nil then error(err) end
		if u == nil then error("expected user, got nil") end
		if u.name ~= "alice" then error("got name: " .. tostring(u.name)) end
	`, "provider_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestProvider_customModuleNameAndBinding(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if _, err := conn.Exec(`CREATE TABLE greetings (msg TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := conn.Exec(`INSERT INTO greetings (msg) VALUES ('hi')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	app := hex.New(hex.WithEnvironment(env.Test))

	// Bind DB under a non-default name to prove DBBinding is honoured.
	app.Singleton("primary_db", func(*container.Container) (any, error) {
		return conn, nil
	})

	err = app.Register(
		&luaprovider.Provider{},
		&dbluaprovider.Provider{
			DBBinding:  "primary_db",
			ModuleName: "primary",
		},
	)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	luaEnv, err := container.Make[*hexlua.Environment](app.Container(), "lua")
	if err != nil {
		t.Fatalf("resolve lua: %v", err)
	}

	err = luaEnv.ExecString(`
		local primary = require("primary")
		local row, err = primary.queryOne("SELECT msg FROM greetings")
		if err ~= nil then error(err) end
		if row.msg ~= "hi" then error("got: " .. tostring(row.msg)) end
	`, "custom_name_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}
