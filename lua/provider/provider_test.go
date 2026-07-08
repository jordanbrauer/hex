package provider_test

import (
	"context"
	"testing"

	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/container"
	hexlua "github.com/jordanbrauer/hex/lua"
	"github.com/jordanbrauer/hex/lua/provider"
	hexprovider "github.com/jordanbrauer/hex/provider"
)

// extenderProvider stands in for a downstream provider (hex/ai/lua,
// third-party plugins, etc.) that resolves the shared env and adds
// its own module / global.
type extenderProvider struct {
	hexprovider.Base
	extended bool
}

func (p *extenderProvider) Register(app hexprovider.Application) error {
	env, err := container.Make[*hexlua.Environment](app, "lua")
	if err != nil {
		return err
	}

	env.PreloadModule("hello", func(L *glua.LState) int {
		mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
			"say": func(L *glua.LState) int {
				L.Push(glua.LString("hi from Go"))

				return 1
			},
		})
		L.Push(mod)

		return 1
	})

	_ = env.SetGlobal("build_version", "test-1.2.3")

	p.extended = true

	return nil
}

func TestProvider_bindsEnvironmentAndSupportsExtensions(t *testing.T) {
	kernel := hex.New()

	ext := &extenderProvider{}

	if err := kernel.Register(&provider.Provider{}, ext); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := kernel.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	t.Cleanup(func() { _ = kernel.Shutdown(context.Background()) })

	if !ext.extended {
		t.Fatalf("extender's Register did not run")
	}

	// Confirm the extension actually reached the shared env.
	envAny, err := kernel.Make("lua")
	if err != nil {
		t.Fatalf("Make lua: %v", err)
	}

	env := envAny.(*hexlua.Environment)

	err = env.ExecString(`
		local hello = require("hello")
		local got = hello.say()
		if got ~= "hi from Go" then error("bad say: " .. got) end

		if build_version ~= "test-1.2.3" then
			error("bad global: " .. tostring(build_version))
		end
	`, "extension_test.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
}

func TestProvider_shutdownClosesEnvironment(t *testing.T) {
	kernel := hex.New()

	p := &provider.Provider{}

	if err := kernel.Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := kernel.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	if err := kernel.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// A second Shutdown should still be a no-op — the provider does
	// not error on closed env.
	if err := kernel.Shutdown(context.Background()); err != nil {
		t.Fatalf("Second Shutdown: %v", err)
	}
}
