// Package fennel makes .fnl (Fennel) source files runnable through
// gopher-lua by embedding the Fennel compiler (a self-contained
// Lua source file distributed by the Fennel project). Consumers do
// not import this package directly; hex/lua's Environment lazy-loads
// it when it encounters a .fnl file.
//
// # Runtime model
//
// The Fennel compiler is itself Lua (fennel.lua, the amalgamation
// build from https://fennel-lang.org). We load it into the same
// gopher-lua state that will execute compiled Fennel output.
// Compilation happens by calling fennel.compileString(src, opts)
// from Go through the state, producing plain Lua source that
// gopher-lua then compiles + executes as normal.
//
// This mirrors hex/lua/teal exactly \u2014 same shape, same API. The
// only differences are the language and the absence of a
// type-checker.
//
// # Framework module access
//
// Any Lua module registered via hex/lua.Environment.PreloadModule
// (db, cache, config, log, env, events, queue, agent, and any
// consumer-added modules) is transparently visible from Fennel
// source. Just require them like normal:
//
//	(local db (require :db))
//	(local rows (db.query "SELECT * FROM users WHERE active = ?" true))
//	(each [_ user (ipairs rows)]
//	  (log.info "user" {:id user.id :name user.name}))
//
// See NOTICE.md for attribution + license terms.
package fennel

import (
	"embed"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

//go:embed fennel.lua
var fs embed.FS

// preloadScript loads the Fennel compiler and stashes the returned
// module in _G.fennel. Idempotent per LState.
const preloadScript = `
	if fennel == nil then
		fennel = require("fennel_bootstrap")
	end
`

// Load installs the Fennel compiler into L. Safe to call multiple
// times; subsequent calls are no-ops.
//
// After Load, the LState exposes:
//
//	fennel        the module table
//	fennel.compileString(src, opts)   Lua source, name-preserving
//	fennel.eval(src, opts)            compile + run
//	fennel.dofile(path)               compile + run a .fnl file
//
// Consumers typically go through Compile/Session below.
func Load(L *lua.LState) error {
	// If already loaded, bail.
	if v := L.GetGlobal("fennel"); v.Type() != lua.LTNil {
		return nil
	}

	src, err := fs.ReadFile("fennel.lua")
	if err != nil {
		return fmt.Errorf("fennel: read embedded source: %w", err)
	}

	// Loading the amalgamation returns the fennel module table.
	// We DoString the source (evaluates the chunk) and assign the
	// return value to _G.fennel.
	//
	// gopher-lua's DoString discards return values, so we can't
	// just DoString + get the return. Instead: wrap the source
	// as a function definition, call it, capture the return.
	wrapped := "return (function()\n" + string(src) + "\nend)()"

	fn, err := L.LoadString(wrapped)
	if err != nil {
		return fmt.Errorf("fennel: load compiler: %w", err)
	}

	L.Push(fn)

	if err := L.PCall(0, 1, nil); err != nil {
		return fmt.Errorf("fennel: init compiler: %w", err)
	}

	mod := L.Get(-1)
	L.Pop(1)

	if mod.Type() != lua.LTTable {
		return errors.New("fennel: compiler returned non-table module")
	}

	L.SetGlobal("fennel", mod)

	return nil
}

// Compile transpiles Fennel source to Lua source. filename is used
// in error messages and (roughly) in source maps.
//
// Non-session Compile compiles each snippet in isolation. Use a
// Session for REPL-style multi-chunk state where prior globals
// should remain visible.
func Compile(L *lua.LState, source, filename string) (string, error) {
	if err := Load(L); err != nil {
		return "", err
	}

	L.SetGlobal("_hex_fennel_src", lua.LString(source))
	L.SetGlobal("_hex_fennel_name", lua.LString(filename))

	defer func() {
		L.SetGlobal("_hex_fennel_src", lua.LNil)
		L.SetGlobal("_hex_fennel_name", lua.LNil)
	}()

	err := L.DoString(`
		local src = _hex_fennel_src
		local name = _hex_fennel_name
		local ok, out = pcall(fennel.compileString, src, {filename = name})
		if not ok then
			error(tostring(out), 0)
		end
		_hex_fennel_out = out
	`)
	if err != nil {
		return "", err
	}

	out := L.GetGlobal("_hex_fennel_out")
	L.SetGlobal("_hex_fennel_out", lua.LNil)

	if out.Type() != lua.LTString {
		return "", errors.New("fennel: no output produced")
	}

	return out.String(), nil
}

// Session wraps an LState for interactive Fennel evaluation. Unlike
// Teal, Fennel doesn't have a persistent type environment to keep
// across compilations \u2014 it's dynamic \u2014 so Session is a thin marker
// that mostly exists for symmetry with teal.Session and to enable
// future features (macro persistence, scope tracking).
//
// Callers should reuse a Session across REPL lines even though the
// runtime state (globals) automatically persists via the shared
// LState.
type Session struct {
	L *lua.LState
}

// NewSession returns a Session bound to L. Calls Load if needed.
func NewSession(L *lua.LState) (*Session, error) {
	if err := Load(L); err != nil {
		return nil, err
	}

	return &Session{L: L}, nil
}

// Compile transpiles source through the session. Currently
// equivalent to the package-level Compile; kept as a method for
// forward compatibility (macro state, persistent scope) and API
// symmetry with hex/lua/teal.
func (s *Session) Compile(source, filename string) (string, error) {
	return Compile(s.L, source, filename)
}

// Close releases any session-scoped resources. No-op in v1 since
// Fennel doesn't allocate a persistent env like Teal does.
func (s *Session) Close() error { return nil }

// IsFennelFile reports whether path has the Fennel file extension.
// hex/lua uses this to auto-route .fnl files through Compile.
func IsFennelFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".fnl")
}
