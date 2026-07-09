package lua_test

import (
	"embed"
	"io/fs"
	"testing"

	"github.com/jordanbrauer/hex/config"
	configlua "github.com/jordanbrauer/hex/config/lua"
	hexlua "github.com/jordanbrauer/hex/lua"
)

//go:embed testdata/*.toml
var testFS embed.FS

func newEnv(t *testing.T) (*hexlua.Environment, *config.Store) {
	t.Helper()

	store, err := config.Load(config.Config{
		Sources:    []fs.FS{testFS},
		SourcesDir: "testdata",
	})
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	bindings := &configlua.Bindings{Store: store}
	env.PreloadModule("config", bindings.Loader)

	return env, store
}

func TestConfig_readTypedValues(t *testing.T) {
	env, _ := newEnv(t)

	err := env.ExecString(`
		local c = require("config")
		if c.string("server.address") ~= ":8080" then error("string") end
		if c.int("db.pool.max_open") ~= 42 then error("int: " .. c.int("db.pool.max_open")) end
		if c.bool("app.debug") ~= true then error("bool") end
		local ms = c.stringSlice("app.mirrors")
		if #ms ~= 3 or ms[1] ~= "a" or ms[3] ~= "c" then error("stringSlice") end
		if c.duration("http.timeout") ~= 30 then error("duration: " .. tostring(c.duration("http.timeout"))) end
	`, "config_read.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestConfig_setAndReadBack(t *testing.T) {
	env, store := newEnv(t)

	err := env.ExecString(`
		local c = require("config")
		local ok, err = c.set("app.name", "renamed")
		if err ~= nil then error(err) end
		if not ok then error("ok=false") end
		if c.string("app.name") ~= "renamed" then error("read: " .. c.string("app.name")) end
	`, "config_set.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	// Confirm from the Go side.
	if got := store.String("app.name"); got != "renamed" {
		t.Errorf("Go side sees %q", got)
	}
}

func TestConfig_hasAndNamespaces(t *testing.T) {
	env, _ := newEnv(t)

	err := env.ExecString(`
		local c = require("config")
		if not c.has("server.address") then error("has: server.address") end
		if c.has("no.such.key") then error("has: no.such.key returned true") end
		local ns = c.namespaces()
		if type(ns) ~= "table" then error("namespaces not a table") end
	`, "config_meta.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}
