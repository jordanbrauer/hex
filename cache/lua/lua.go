// Package lua exposes hex/cache to Lua scripts as the "cache"
// module.
//
//	local cache = require("cache")
//
//	local ok, err = cache.set("greeting", "hi")             -- no TTL
//	local ok, err = cache.set("hot", "value", 60)           -- 60s TTL
//	local v, hit, err = cache.get("greeting")
//	local ok = cache.has("greeting")
//	cache.delete("greeting")
//	cache.clear()                                            -- driver may refuse
//	local n, err = cache.increment("counter", 1)
//
// Values are strings on the Lua side; hex/cache stores raw []byte.
// Structured values should be JSON-encoded before caching — a
// gopher-json module is on the follow-up list.
package lua

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/cache"
)

// TypeStub is the Teal .d.tl source describing the `cache` module.
//
//go:embed cache.d.tl
var TypeStub string

// Bindings configures the 'cache' module.
type Bindings struct {
	// Cache is the backend to read/write. Required.
	Cache cache.Cache

	// Context, when non-nil, is used for every cache call.
	// Defaults to context.Background().
	Context context.Context
}

// Loader is the gopher-lua LGFunction registered against
// env.PreloadModule("cache", b.Loader).
func (b *Bindings) Loader(L *glua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
		"get":       b.luaGet,
		"set":       b.luaSet,
		"delete":    b.luaDelete,
		"has":       b.luaHas,
		"clear":     b.luaClear,
		"increment": b.luaIncrement,
	})
	L.Push(mod)

	return 1
}

func (b *Bindings) ctx() context.Context {
	if b.Context != nil {
		return b.Context
	}

	return context.Background()
}

// luaGet: cache.get(key) -> (value, hit, err)
func (b *Bindings) luaGet(L *glua.LState) int {
	if b.Cache == nil {
		return pushErr3(L, "cache.get: no backend configured")
	}

	key := L.CheckString(1)

	v, hit, err := b.Cache.Get(b.ctx(), key)
	if err != nil {
		return pushErr3(L, fmt.Sprintf("cache.get: %v", err))
	}

	if !hit {
		L.Push(glua.LNil)
		L.Push(glua.LBool(false))
		L.Push(glua.LNil)

		return 3
	}

	L.Push(glua.LString(v))
	L.Push(glua.LBool(true))
	L.Push(glua.LNil)

	return 3
}

// luaSet: cache.set(key, value, ttl_seconds?) -> (true, nil) | (nil, err)
func (b *Bindings) luaSet(L *glua.LState) int {
	if b.Cache == nil {
		return pushErr2(L, "cache.set: no backend configured")
	}

	key := L.CheckString(1)
	value := L.CheckString(2)

	var ttl time.Duration
	if L.GetTop() >= 3 {
		ttl = time.Duration(L.CheckNumber(3) * glua.LNumber(time.Second))
	}

	if err := b.Cache.Set(b.ctx(), key, []byte(value), ttl); err != nil {
		return pushErr2(L, fmt.Sprintf("cache.set: %v", err))
	}

	L.Push(glua.LBool(true))
	L.Push(glua.LNil)

	return 2
}

// luaDelete: cache.delete(key) -> (true, nil) | (nil, err)
func (b *Bindings) luaDelete(L *glua.LState) int {
	if b.Cache == nil {
		return pushErr2(L, "cache.delete: no backend configured")
	}

	key := L.CheckString(1)

	if err := b.Cache.Delete(b.ctx(), key); err != nil {
		return pushErr2(L, fmt.Sprintf("cache.delete: %v", err))
	}

	L.Push(glua.LBool(true))
	L.Push(glua.LNil)

	return 2
}

// luaHas: cache.has(key) -> (bool, err?)
func (b *Bindings) luaHas(L *glua.LState) int {
	if b.Cache == nil {
		return pushErr2(L, "cache.has: no backend configured")
	}

	key := L.CheckString(1)

	ok, err := b.Cache.Has(b.ctx(), key)
	if err != nil {
		return pushErr2(L, fmt.Sprintf("cache.has: %v", err))
	}

	L.Push(glua.LBool(ok))
	L.Push(glua.LNil)

	return 2
}

// luaClear: cache.clear() -> (true, nil) | (nil, err)
//
// Driver may refuse (Redis / production) and return an error;
// that's honoured here.
func (b *Bindings) luaClear(L *glua.LState) int {
	if b.Cache == nil {
		return pushErr2(L, "cache.clear: no backend configured")
	}

	if err := b.Cache.Clear(b.ctx()); err != nil {
		return pushErr2(L, fmt.Sprintf("cache.clear: %v", err))
	}

	L.Push(glua.LBool(true))
	L.Push(glua.LNil)

	return 2
}

// luaIncrement: cache.increment(key, delta) -> (n, nil) | (nil, err)
func (b *Bindings) luaIncrement(L *glua.LState) int {
	if b.Cache == nil {
		return pushErr2(L, "cache.increment: no backend configured")
	}

	key := L.CheckString(1)
	delta := int64(L.CheckNumber(2))

	n, err := b.Cache.Increment(b.ctx(), key, delta)
	if err != nil {
		return pushErr2(L, fmt.Sprintf("cache.increment: %v", err))
	}

	L.Push(glua.LNumber(n))
	L.Push(glua.LNil)

	return 2
}

func pushErr2(L *glua.LState, msg string) int {
	L.Push(glua.LNil)
	L.Push(glua.LString(msg))

	return 2
}

func pushErr3(L *glua.LState, msg string) int {
	L.Push(glua.LNil)
	L.Push(glua.LNil)
	L.Push(glua.LString(msg))

	return 3
}
