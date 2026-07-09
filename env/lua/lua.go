// Package lua exposes hex/env to Lua scripts as the "env" module.
//
//	local env = require("env")
//
//	print(env.current())         -- "development" | "test" | "production"
//	if env.is_production() then
//	    -- guard destructive writes, etc.
//	end
//	if env.is_test() then ... end
//	if env.is_development() then ... end
//
// The module is stateless once bound; the current environment is
// captured at install time from *hex.App.Environment(). Apps that
// hot-swap their environment mid-run (rare) can reinstall.
package lua

import (
	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/env"
)

// Bindings configures the 'env' module. Environment is captured at
// module install time — usually from *hex.App.Environment() —
// meaning subsequent reads report the value it had at that moment.
type Bindings struct {
	// Environment is the runtime environment reported by env.current().
	// Required.
	Environment env.Environment
}

// Loader is the gopher-lua LGFunction registered against
// env.PreloadModule("env", b.Loader).
func (b *Bindings) Loader(L *glua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
		"current":        b.luaCurrent,
		"is_development": b.luaIsDev,
		"is_test":        b.luaIsTest,
		"is_production":  b.luaIsProd,
	})

	// Also expose the environment string directly for interpolation:
	//   log.info("running", { env = env.name })
	L.SetField(mod, "name", glua.LString(string(b.Environment)))

	L.Push(mod)

	return 1
}

func (b *Bindings) luaCurrent(L *glua.LState) int {
	L.Push(glua.LString(string(b.Environment)))

	return 1
}

func (b *Bindings) luaIsDev(L *glua.LState) int {
	L.Push(glua.LBool(b.Environment.IsDev()))

	return 1
}

func (b *Bindings) luaIsTest(L *glua.LState) int {
	L.Push(glua.LBool(b.Environment.IsTest()))

	return 1
}

func (b *Bindings) luaIsProd(L *glua.LState) int {
	L.Push(glua.LBool(b.Environment.IsProduction()))

	return 1
}
