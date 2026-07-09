// Package provider is the default hex/lua service provider.
//
// It constructs a shared *lua.Environment during Register and binds
// it into the container under "lua" so other providers can resolve
// it and contribute modules, globals, or preloaded scripts:
//
//	// In another provider's Register:
//	env, _ := container.Make[*hexlua.Environment](app, "lua")
//	env.PreloadModule("myapp", myModule.Loader)
//	env.SetGlobal("build_version", "v1.2.3")
//
// As a foundational convenience, this provider also installs the
// 'config' and 'log' Lua modules automatically — both are
// always-available in a hex app, so binding them here avoids
// forcing every scaffold to wire dedicated bridge providers for
// them. Other services (db, cache, queue, events, ai, ...) are
// optional; their own providers install their Lua modules when
// they see a shared env in the container.
//
// Register order: hex/config/provider first (its "config" binding
// is resolved here), then hex/log/provider, then this provider.
//
// Shutdown closes the environment.
package provider

import (
	"context"
	"embed"
	"io/fs"

	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/config"
	configlua "github.com/jordanbrauer/hex/config/lua"
	"github.com/jordanbrauer/hex/container"
	envlua "github.com/jordanbrauer/hex/env/lua"
	eventslua "github.com/jordanbrauer/hex/events/lua"
	loglua "github.com/jordanbrauer/hex/log/lua"
	hexlua "github.com/jordanbrauer/hex/lua"
	"github.com/jordanbrauer/hex/provider"
)

//go:embed config
var configFS embed.FS

// Configs returns the embedded default TOML + CUE files this provider
// contributes to hex/config. Add it to hex/config.Provider.Sources to
// pick up the "repl" namespace (mode = "teal" | "lua").
func Configs() fs.FS {
	sub, err := fs.Sub(configFS, "config")
	if err != nil {
		panic("lua/provider: embedded config subdir missing: " + err.Error())
	}

	return sub
}

// Provider wires a shared *hex/lua.Environment into the container.
// Other providers register modules/globals against it in their own
// Register phase.
type Provider struct {
	provider.Base

	// Binding is the container key for the environment. Defaults to
	// "lua".
	Binding string

	// Options are passed verbatim to hexlua.New.
	Options []hexlua.Option

	env *hexlua.Environment
}

// Register builds the environment, binds it into the container, and
// preloads the foundational `config` and `log` Lua modules.
func (p *Provider) Register(app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "lua"
	}

	p.env = hexlua.New(p.Options...)

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.env, nil
	})

	p.installConfigModule(app)
	p.installLogModule()
	p.installEnvModule(app)
	p.installEventsModule(app)

	return nil
}

// installConfigModule preloads the 'config' Lua module against the
// shared env. The *config.Store must already be bound in the
// container — the scaffolder registers hex/config/provider before
// this one. Silent no-op when the binding is missing, so that a
// test that only wants hex/lua without hex/config still works.
func (p *Provider) installConfigModule(app provider.Application) {
	store, err := container.Make[*config.Store](app, "config")
	if err != nil || store == nil {
		return
	}

	bindings := &configlua.Bindings{Store: store}

	p.env.SetType("config", configlua.TypeStub)

	p.env.PreloadModule("config", func(L *glua.LState) int {
		return bindings.Loader(L)
	})
}

// installEnvModule preloads the 'env' Lua module. The current
// environment is captured from app.Environment() at install time.
func (p *Provider) installEnvModule(app provider.Application) {
	bindings := &envlua.Bindings{Environment: app.Environment()}

	p.env.SetType("env", envlua.TypeStub)

	p.env.PreloadModule("env", func(L *glua.LState) int {
		return bindings.Loader(L)
	})
}

// installEventsModule preloads the 'events' Lua module. Uses the
// provider.Application itself as the emitter — it forwards to the
// app's events.Bus, so Lua-emitted events reach Go subscribers
// registered via app.On.
func (p *Provider) installEventsModule(app provider.Application) {
	bindings := &eventslua.Bindings{Emitter: app}

	p.env.SetType("events", eventslua.TypeStub)

	p.env.PreloadModule("events", func(L *glua.LState) int {
		return bindings.Loader(L)
	})
}

// installLogModule preloads the 'log' Lua module. hex/log delegates
// to slog.Default at call time, so no container lookup is needed —
// the module is stateless.
func (p *Provider) installLogModule() {
	bindings := &loglua.Bindings{}

	p.env.SetType("log", loglua.TypeStub)

	p.env.PreloadModule("log", func(L *glua.LState) int {
		return bindings.Loader(L)
	})
}

// Shutdown closes the shared Lua environment.
func (p *Provider) Shutdown(_ context.Context, _ provider.Application) error {
	if p.env == nil {
		return nil
	}

	return p.env.Close()
}

// Compile-time confirmation the provider participates in shutdown.
var _ provider.Shutdowner = (*Provider)(nil)
