// Package lua exposes hex/log to Lua scripts as the "log" module.
//
//	local log = require("log")
//
//	log.info("hello",  { user = "alice" })
//	log.debug("query", { sql = "SELECT ..." })
//	log.warn("slow",   { ms = 823 })
//	log.error("boom",  { err = tostring(err) })
//
// The optional second argument is a table of key/value attributes,
// forwarded to slog / hex/log as structured fields. Keys should be
// strings; values are converted with the same rules used by
// hex/db/lua and hex/config/lua (numbers, strings, bools, nil,
// nested tables become their string form).
//
// The module delegates to hex/log's package-level functions, which
// in turn use slog.Default. This lets tests swap the global handler
// via slog.SetDefault and see Lua-side log calls captured.
package lua

import (
	glua "github.com/yuin/gopher-lua"

	hexlog "github.com/jordanbrauer/hex/log"
)

// Bindings is intentionally empty — the log module has no state to
// carry (slog.Default is a global). The type exists for symmetry
// with the other Lua bridges and to keep the Loader signature
// method-shaped.
type Bindings struct{}

// Loader is the gopher-lua LGFunction registered against
// env.PreloadModule("log", b.Loader).
func (b *Bindings) Loader(L *glua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
		"debug": b.luaDebug,
		"info":  b.luaInfo,
		"warn":  b.luaWarn,
		"error": b.luaError,
	})
	L.Push(mod)

	return 1
}

func (b *Bindings) luaDebug(L *glua.LState) int { return b.dispatch(L, hexlog.Debug) }
func (b *Bindings) luaInfo(L *glua.LState) int  { return b.dispatch(L, hexlog.Info) }
func (b *Bindings) luaWarn(L *glua.LState) int  { return b.dispatch(L, hexlog.Warn) }
func (b *Bindings) luaError(L *glua.LState) int { return b.dispatch(L, hexlog.Error) }

// dispatch reads the message + optional attrs table and forwards to
// the given hex/log function. Attrs are flattened into key/value
// pairs (slog-style variadic args).
func (b *Bindings) dispatch(L *glua.LState, fn func(msg string, args ...any)) int {
	msg := L.CheckString(1)

	var args []any

	if L.GetTop() >= 2 {
		if tbl, ok := L.Get(2).(*glua.LTable); ok {
			tbl.ForEach(func(k, v glua.LValue) {
				args = append(args, k.String(), luaToGo(v))
			})
		}
	}

	fn(msg, args...)

	return 0
}

// luaToGo converts a Lua value to a Go value for slog attribute
// values. Kept in-package for the same reasons as hex/db/lua and
// hex/config/lua — avoid a shared helper package pull for a trivial
// function.
func luaToGo(v glua.LValue) any {
	switch t := v.(type) {
	case glua.LBool:
		return bool(t)
	case glua.LNumber:
		n := float64(t)
		if n == float64(int64(n)) {
			return int64(n)
		}

		return n
	case glua.LString:
		return string(t)
	case *glua.LNilType:
		return nil
	default:
		return v.String()
	}
}
