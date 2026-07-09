package lua_test

import (
	"testing"

	"github.com/jordanbrauer/hex/env"
	envlua "github.com/jordanbrauer/hex/env/lua"
	hexlua "github.com/jordanbrauer/hex/lua"
)

func newEnv(t *testing.T, e env.Environment) *hexlua.Environment {
	t.Helper()

	lenv := hexlua.New()
	t.Cleanup(func() { _ = lenv.Close() })

	b := &envlua.Bindings{Environment: e}
	lenv.PreloadModule("env", b.Loader)

	return lenv
}

func TestEnv_current(t *testing.T) {
	cases := []struct {
		e    env.Environment
		want string
	}{
		{env.Development, "development"},
		{env.Test, "test"},
		{env.Production, "production"},
	}

	for _, tc := range cases {
		t.Run(string(tc.e), func(t *testing.T) {
			l := newEnv(t, tc.e)

			err := l.ExecString(`
				local env = require("env")
				local got = env.current()
				if got ~= "`+tc.want+`" then
					error("current=" .. got)
				end
				if env.name ~= "`+tc.want+`" then
					error("name=" .. env.name)
				end
			`, "env_current.lua")
			if err != nil {
				t.Fatalf("Exec: %v", err)
			}
		})
	}
}

func TestEnv_predicates(t *testing.T) {
	l := newEnv(t, env.Production)

	err := l.ExecString(`
		local env = require("env")
		if not env.is_production() then error("is_production=false") end
		if env.is_development() then error("is_development=true") end
		if env.is_test() then error("is_test=true") end
	`, "env_pred.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}
