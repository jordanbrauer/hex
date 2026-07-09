// Package lua exposes hex/config to Lua scripts as the "config"
// module.
//
//	local config = require("config")
//
//	local port = config.string("server.address")
//	local n    = config.int("db.pool.max_open")
//	local dbg  = config.bool("app.debug")
//	local t    = config.duration("http.timeout")   -- returns seconds (number)
//	local xs   = config.stringSlice("app.mirrors")
//
//	config.set("app.name", "reset from REPL")     -- runtime override
//	local exists = config.has("some.key")
//
//	for _, ns in ipairs(config.namespaces()) do
//	    print(ns)
//	end
//
// The `duration` helper returns the value as seconds (Lua number).
// Convert to milliseconds by multiplying by 1000 if needed.
//
// Errors return (nil, "message"). config.set surfaces validation
// failures against the CUE schema when applicable.
package lua

import (
	"fmt"

	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/config"
)

// Bindings configures the 'config' module.
type Bindings struct {
	// Store is the config store to read/write. Required.
	Store *config.Store
}

// Loader is the gopher-lua LGFunction registered against
// env.PreloadModule("config", b.Loader).
func (b *Bindings) Loader(L *glua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
		"string":      b.luaString,
		"int":         b.luaInt,
		"bool":        b.luaBool,
		"float":       b.luaFloat,
		"duration":    b.luaDuration,
		"stringSlice": b.luaStringSlice,
		"set":         b.luaSet,
		"has":         b.luaHas,
		"namespaces":  b.luaNamespaces,
	})
	L.Push(mod)

	return 1
}

func (b *Bindings) luaString(L *glua.LState) int {
	L.Push(glua.LString(b.Store.String(L.CheckString(1))))

	return 1
}

func (b *Bindings) luaInt(L *glua.LState) int {
	L.Push(glua.LNumber(b.Store.Int(L.CheckString(1))))

	return 1
}

func (b *Bindings) luaBool(L *glua.LState) int {
	L.Push(glua.LBool(b.Store.Bool(L.CheckString(1))))

	return 1
}

func (b *Bindings) luaFloat(L *glua.LState) int {
	L.Push(glua.LNumber(b.Store.Float64(L.CheckString(1))))

	return 1
}

// luaDuration returns the duration as seconds (Lua number). Callers
// converting to nanoseconds or milliseconds do the arithmetic in
// Lua — keeps the surface small.
func (b *Bindings) luaDuration(L *glua.LState) int {
	d := b.Store.Duration(L.CheckString(1))
	L.Push(glua.LNumber(d.Seconds()))

	return 1
}

func (b *Bindings) luaStringSlice(L *glua.LState) int {
	tbl := L.NewTable()
	for i, v := range b.Store.StringSlice(L.CheckString(1)) {
		tbl.RawSetInt(i+1, glua.LString(v))
	}
	L.Push(tbl)

	return 1
}

// luaSet writes a runtime override. Returns (true, nil) on success
// or (nil, err) on validation / write failure.
func (b *Bindings) luaSet(L *glua.LState) int {
	key := L.CheckString(1)
	value := luaToGo(L.Get(2))

	if err := b.Store.Set(key, value); err != nil {
		L.Push(glua.LNil)
		L.Push(glua.LString(fmt.Sprintf("config.set: %v", err)))

		return 2
	}

	L.Push(glua.LBool(true))
	L.Push(glua.LNil)

	return 2
}

func (b *Bindings) luaHas(L *glua.LState) int {
	L.Push(glua.LBool(b.Store.Has(L.CheckString(1))))

	return 1
}

func (b *Bindings) luaNamespaces(L *glua.LState) int {
	tbl := L.NewTable()
	for i, ns := range b.Store.Namespaces() {
		tbl.RawSetInt(i+1, glua.LString(ns))
	}
	L.Push(tbl)

	return 1
}

// luaToGo converts a Lua value to a Go value for config.Store.Set.
// Kept in-package to avoid depending on hex/db/lua's copy.
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
