// Package teal makes .tl (Teal) source files runnable through
// gopher-lua by embedding the Teal compiler + Lua 5.2 compatibility
// shims. Consumers do not import this package directly; hex/lua's
// Environment lazy-loads it when it encounters a .tl file.
//
// # Runtime model
//
// Teal's compiler is itself Lua (tl.lua). We load it into the same
// gopher-lua state that will execute compiled Teal output. Compilation
// happens by calling tl.process(filename) + tl.pretty_print_ast(ast,
// "5.1") from Go through the state, producing plain Lua source that
// gopher-lua then compiles + executes as normal.
//
// See NOTICE.md for attribution + the patch trail on the vendored
// Lua files.
package teal

import (
	"embed"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

//go:embed tl.lua compat52.lua bit.lua bit32.lua
var fs embed.FS

// preload files, order matters: compat52 first so tl.lua can rely on
// Lua 5.2 primitives when it loads.
var preloadFiles = []string{"bit.lua", "bit32.lua", "compat52.lua", "tl.lua"}

// bootstrap installs the tl global. Runs once per state after all
// preload registrations complete.
const bootstrap = `require('compat52'); tl = require('tl'); tl.cache = {}`

// Load installs the Teal compiler into L. Subsequent calls to Compile
// / Check / Exec against the same state reuse the loaded compiler.
// Safe to call more than once — the bootstrap script is idempotent
// (re-assigning tl and clearing tl.cache).
func Load(L *lua.LState) error {
	for _, name := range preloadFiles {
		if err := preload(L, name); err != nil {
			return fmt.Errorf("teal: preload %s: %w", name, err)
		}
	}

	if err := L.DoString(bootstrap); err != nil {
		return fmt.Errorf("teal: bootstrap: %w", err)
	}

	return nil
}

// Compile transpiles a Teal source string into Lua source. filename
// is used only for error reporting.
//
// L must have had Load called on it beforehand.
func Compile(L *lua.LState, source, filename string) (string, error) {
	// Push source + filename onto the stack via globals to avoid
	// escaping Lua strings by hand.
	L.SetGlobal("_hex_teal_src", lua.LString(source))
	L.SetGlobal("_hex_teal_name", lua.LString(filename))

	defer func() {
		L.SetGlobal("_hex_teal_src", lua.LNil)
		L.SetGlobal("_hex_teal_name", lua.LNil)
	}()

	err := L.DoString(`
		local src = _hex_teal_src
		local name = _hex_teal_name
		local result, perr = tl.process_string(src, false, nil, nil, nil, name)
		if perr ~= nil then
			error(name .. ': teal process: ' .. tostring(perr), 0)
		end
		if result.syntax_errors and #result.syntax_errors > 0 then
			local e = result.syntax_errors[1]
			error((e.filename or name) .. ':' .. tostring(e.y) .. ': ' .. tostring(e.msg), 0)
		end
		if result.type_errors and #result.type_errors > 0 then
			local e = result.type_errors[1]
			error((e.filename or name) .. ':' .. tostring(e.y) .. ': type error: ' .. tostring(e.msg), 0)
		end
		local code, gerr = tl.pretty_print_ast(result.ast, "5.1")
		if gerr ~= nil then
			error(name .. ': teal codegen: ' .. tostring(gerr), 0)
		end
		_hex_teal_out = code
	`)
	if err != nil {
		return "", err
	}

	out := L.GetGlobal("_hex_teal_out")
	L.SetGlobal("_hex_teal_out", lua.LNil)

	if out.Type() != lua.LTString {
		return "", errors.New("teal: no output produced")
	}

	return out.String(), nil
}

// Check runs tl.process on the source but stops before code
// generation, returning any syntax + type errors found. Useful for
// CI: fail the build on Teal type errors without executing anything.
func Check(L *lua.LState, source, filename string) error {
	L.SetGlobal("_hex_teal_src", lua.LString(source))
	L.SetGlobal("_hex_teal_name", lua.LString(filename))

	defer func() {
		L.SetGlobal("_hex_teal_src", lua.LNil)
		L.SetGlobal("_hex_teal_name", lua.LNil)
	}()

	err := L.DoString(`
		local src = _hex_teal_src
		local name = _hex_teal_name
		local result, perr = tl.process_string(src, false, nil, nil, nil, name)
		if perr ~= nil then
			error(name .. ': teal process: ' .. tostring(perr), 0)
		end
		local errs = {}
		if result.syntax_errors then
			for _, e in ipairs(result.syntax_errors) do
				errs[#errs+1] = (e.filename or name) .. ':' .. tostring(e.y) .. ': ' .. tostring(e.msg)
			end
		end
		if result.type_errors then
			for _, e in ipairs(result.type_errors) do
				errs[#errs+1] = (e.filename or name) .. ':' .. tostring(e.y) .. ': type error: ' .. tostring(e.msg)
			end
		end
		if #errs > 0 then
			error(table.concat(errs, "\n"), 0)
		end
	`)

	return err
}

// preload reads a vendored .lua file and stashes it under
// package.preload[<basename>], so a subsequent require('<basename>')
// picks up the embedded copy.
func preload(L *lua.LState, filename string) error {
	pkg := strings.TrimSuffix(filename, filepath.Ext(filename))

	data, err := fs.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}

	mod, err := L.LoadString(string(data))
	if err != nil {
		return fmt.Errorf("compile %s: %w", filename, err)
	}

	pkgTbl := L.GetField(L.Get(lua.EnvironIndex), "package")
	if pkgTbl == lua.LNil {
		return errors.New("no 'package' table in Lua state")
	}

	preloadTbl := L.GetField(pkgTbl, "preload")
	if preloadTbl == lua.LNil {
		return errors.New("no 'package.preload' table in Lua state")
	}

	L.SetField(preloadTbl, pkg, mod)

	return nil
}
