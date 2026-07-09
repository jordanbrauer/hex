// Package provider is the service provider that installs the hex/db
// 'db' Lua module into the shared hex/lua environment.
//
// Add to app/boot.go AFTER both hex/db/provider (binds "db") and
// hex/lua/provider (binds "lua"):
//
//	provider.Config(),
//	provider.Log(),
//	provider.Lua(),        // hex/lua/provider
//	provider.Database(),   // hex/db/provider
//	provider.LuaDB(),      // this package — bridges the two
//
// After boot, any Lua script (including the REPL) can:
//
//	local db = require("db")
//	local rows, err = db.query("SELECT * FROM users WHERE id = ?", 1)
package provider

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/container"
	dblua "github.com/jordanbrauer/hex/db/lua"
	hexlua "github.com/jordanbrauer/hex/lua"
	"github.com/jordanbrauer/hex/provider"
)

// Provider wires the 'db' Lua module into hex/lua's shared env.
type Provider struct {
	provider.Base

	// LuaBinding is the container key for the *hex/lua.Environment.
	// Defaults to "lua".
	LuaBinding string

	// DBBinding is the container key for the *sql.DB. Defaults to
	// "db".
	DBBinding string

	// ModuleName is the Lua module name registered via
	// env.PreloadModule. Defaults to "db". Change when the consumer
	// wants to expose multiple databases under distinct module
	// names.
	ModuleName string

	// Mutex, when non-nil, is locked around every DB call from Lua.
	// Nil is fine for the REPL (single-threaded) and event handlers
	// that already serialise access to their LState. Provide one
	// when many goroutines share an LState.
	Mutex *sync.Mutex
}

// Register resolves both bindings and installs the module.
//
// The *sql.DB binding may not resolve at Register time —
// hex/db/provider opens the actual connection in Boot. We register a
// loader closure that resolves lazily on first require("db"), by
// which point Boot has run.
func (p *Provider) Register(app provider.Application) error {
	luaBinding := p.LuaBinding
	if luaBinding == "" {
		luaBinding = "lua"
	}

	dbBinding := p.DBBinding
	if dbBinding == "" {
		dbBinding = "db"
	}

	moduleName := p.ModuleName
	if moduleName == "" {
		moduleName = "db"
	}

	env, err := container.Make[*hexlua.Environment](app, luaBinding)
	if err != nil {
		return fmt.Errorf("db/lua/provider: resolve %q: %w", luaBinding, err)
	}

	if env == nil {
		return errors.New("db/lua/provider: hex/lua environment is nil (register hex/lua/provider first)")
	}

	bindings := &dblua.Bindings{Mutex: p.Mutex}

	// Lazy resolve at first require("db"): the DB provider registers a
	// singleton whose factory returns an error until Boot runs. If we
	// resolve eagerly here (in Register), the singleton will cache
	// that error forever — even after Boot successfully opens the
	// connection — poisoning every subsequent lookup. So we defer
	// entirely to require() time; by then Boot has completed.
	env.PreloadModule(moduleName, func(L *glua.LState) int {
		if bindings.DB == nil {
			db, err := container.Make[*sql.DB](app, dbBinding)
			if err != nil {
				L.RaiseError("db/lua/provider: resolve %q: %v", dbBinding, err)

				return 0
			}

			bindings.DB = db
		}

		return bindings.Loader(L)
	})

	return nil
}

// Compile-time confirmation this satisfies the standard Service
// interface. Shutdown is not needed — the shared env and DB are
// closed by their owning providers.
var _ provider.Service = (*Provider)(nil)
