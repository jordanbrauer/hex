// Package provider is the default hex/lua service provider.
//
// It constructs a shared *lua.Environment during Register and binds it
// into the container under "lua" so other providers can resolve it
// and contribute modules, globals, or preloaded scripts:
//
//	// In another provider's Register:
//	env, _ := container.Make[*hexlua.Environment](app, "lua")
//	env.PreloadModule("myapp", myModule.Loader)
//	env.SetGlobal("build_version", "v1.2.3")
//
// This matches how hex/web exposes *web.Server for route registration
// and how hex/config accepts Sources — the framework provides the
// primitive; other providers layer on top.
//
// Shutdown closes the environment.
package provider

import (
	"context"

	hexlua "github.com/jordanbrauer/hex/lua"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/provider"
)

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

// Register builds the environment and binds it.
func (p *Provider) Register(app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "lua"
	}

	p.env = hexlua.New(p.Options...)

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.env, nil
	})

	return nil
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
