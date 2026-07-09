package lua_test

import (
	"testing"

	"github.com/jordanbrauer/hex/cache/memory"

	cachelua "github.com/jordanbrauer/hex/cache/lua"
	hexlua "github.com/jordanbrauer/hex/lua"
)

func newEnv(t *testing.T) *hexlua.Environment {
	t.Helper()

	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	b := &cachelua.Bindings{Cache: memory.New()}
	env.PreloadModule("cache", b.Loader)

	return env
}

func TestCache_setGetDelete(t *testing.T) {
	env := newEnv(t)

	err := env.ExecString(`
		local cache = require("cache")
		local ok, err = cache.set("k", "hello")
		if err ~= nil then error(err) end
		if not ok then error("set ok=false") end

		local v, hit, err = cache.get("k")
		if err ~= nil then error(err) end
		if not hit then error("miss on freshly-set key") end
		if v ~= "hello" then error("v=" .. tostring(v)) end

		cache.delete("k")

		local _, hit2 = cache.get("k")
		if hit2 then error("still hit after delete") end
	`, "cache_setget.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestCache_hasAndIncrement(t *testing.T) {
	env := newEnv(t)

	err := env.ExecString(`
		local cache = require("cache")
		if cache.has("x") then error("has=true before set") end

		local n, err = cache.increment("counter", 5)
		if err ~= nil then error(err) end
		if n ~= 5 then error("first inc n=" .. tostring(n)) end

		local n2 = cache.increment("counter", 3)
		if n2 ~= 8 then error("second inc n=" .. tostring(n2)) end

		if not cache.has("counter") then error("has=false after inc") end
	`, "cache_meta.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestCache_clear(t *testing.T) {
	env := newEnv(t)

	err := env.ExecString(`
		local cache = require("cache")
		cache.set("a", "1")
		cache.set("b", "2")
		local ok, err = cache.clear()
		if err ~= nil then error(err) end
		if not ok then error("clear ok=false") end

		local _, hit_a = cache.get("a")
		local _, hit_b = cache.get("b")
		if hit_a or hit_b then error("keys still present after clear") end
	`, "cache_clear.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}
