// Package lua exposes hex/events to Lua scripts as the "events"
// module.
//
//	local events = require("events")
//
//	events.emit("user.created", { id = 1, name = "alice" })
//	events.emit("app.ready")   -- no payload
//
// v1 is emit-only. Subscribing from Lua ("events.on(...)") is a
// follow-up: cross-goroutine callbacks into an LState need careful
// serialisation (the LState is not thread-safe), and the REPL /
// scripting use cases don't need it yet. Go-side subscribers
// registered via app.On see Lua-emitted events immediately.
package lua

import (
	"fmt"

	glua "github.com/yuin/gopher-lua"
)

// Emitter is the small surface events.Bus (and *hex.App) expose to
// this module. Any type with Emit(event, data ...any) error works.
type Emitter interface {
	Emit(event string, data ...any) error
}

// Bindings configures the 'events' module.
type Bindings struct {
	// Emitter is the target for events.emit(). Required. Typically
	// *hex.App itself (which delegates to its *events.Bus).
	Emitter Emitter
}

// Loader is the gopher-lua LGFunction registered against
// env.PreloadModule("events", b.Loader).
func (b *Bindings) Loader(L *glua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
		"emit": b.luaEmit,
	})
	L.Push(mod)

	return 1
}

// luaEmit: events.emit(name, payload?) -> (true, nil) | (nil, err)
//
// The payload, when supplied, is passed to Go-side subscribers as
// the first vararg to their Subscriber(data ...any) function.
// Lua tables are converted to map[string]any / []any recursively so
// subscribers see idiomatic Go data.
func (b *Bindings) luaEmit(L *glua.LState) int {
	if b.Emitter == nil {
		L.Push(glua.LNil)
		L.Push(glua.LString("events.emit: no emitter configured"))

		return 2
	}

	name := L.CheckString(1)

	var payload any
	if L.GetTop() >= 2 {
		payload = luaToGo(L.Get(2))
	}

	if err := b.Emitter.Emit(name, payload); err != nil {
		L.Push(glua.LNil)
		L.Push(glua.LString(fmt.Sprintf("events.emit: %v", err)))

		return 2
	}

	L.Push(glua.LBool(true))
	L.Push(glua.LNil)

	return 2
}

// luaToGo converts a Lua value to a Go value for emission. Tables
// become map[string]any when they have any string keys, otherwise
// []any (array-like). Nested tables are converted recursively.
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
	case *glua.LTable:
		return tableToGo(t)
	default:
		return v.String()
	}
}

// tableToGo converts a Lua table to either []any (pure-array) or
// map[string]any (any string key present).
func tableToGo(tbl *glua.LTable) any {
	hasString := false

	tbl.ForEach(func(k glua.LValue, _ glua.LValue) {
		if _, ok := k.(glua.LString); ok {
			hasString = true
		}
	})

	if hasString {
		out := map[string]any{}
		tbl.ForEach(func(k, v glua.LValue) {
			out[k.String()] = luaToGo(v)
		})

		return out
	}

	// Array-like: read 1..n while there are values.
	var out []any
	for i := 1; ; i++ {
		v := tbl.RawGetInt(i)
		if v == glua.LNil {
			break
		}

		out = append(out, luaToGo(v))
	}

	return out
}
