// Package provider is the default hex/web service provider.
//
// It reads server config, constructs a hex/web.Server, binds it into
// the container under "http", starts it in a background goroutine
// during Boot, and gracefully shuts it down during Shutdown.
//
// Consumers register routes from another provider that resolves "http"
// out of the container during its own Boot, or by holding a reference
// to the *Server via a factory-level closure.
package provider

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/jordanbrauer/hex/config"
	"github.com/jordanbrauer/hex/container"
	hexlog "github.com/jordanbrauer/hex/log"
	"github.com/jordanbrauer/hex/provider"
	"github.com/jordanbrauer/hex/web"
)

//go:embed config
var configFS embed.FS

// Configs returns the embedded default TOML + CUE files this provider
// contributes to hex/config. Add it to hex/config.Provider.Sources.
func Configs() fs.FS {
	sub, err := fs.Sub(configFS, "config")
	if err != nil {
		panic("web/provider: embedded config subdir missing: " + err.Error())
	}

	return sub
}

// Provider wires a *web.Server into the container.
type Provider struct {
	provider.Base

	// Binding is the container name. Defaults to "http".
	Binding string

	// Namespace is the config namespace read for server settings.
	// Defaults to "server".
	Namespace string

	// ExtraOptions extend the web.Options built from config.
	ExtraOptions web.Options

	// PublicDir, when non-empty, is served at / via echo.Static. Use
	// this to expose a compiled `public/` directory of CSS, JS, and
	// images alongside your Go routes. Route order matters — static
	// files are matched by path, so explicit routes registered on
	// the same path (e.g. `/`) win.
	PublicDir string

	// Configure runs after Register — receives the Server so consumer
	// providers can register routes and middleware before Boot starts
	// the listener.
	Configure func(*web.Server) error

	server  *web.Server
	errored chan error
}

// Register constructs the Server and binds it.
func (p *Provider) Register(app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "http"
	}

	store, err := container.Make[*config.Store](app, "config")
	if err != nil {
		return fmt.Errorf("web/provider: resolve config: %w", err)
	}

	opts := p.buildOptions(store)
	p.server = web.New(opts)

	if p.PublicDir != "" {
		p.server.Echo().Static("/", p.PublicDir)
	}

	if p.Configure != nil {
		if err := p.Configure(p.server); err != nil {
			return err
		}
	}

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.server, nil
	})

	return nil
}

// Boot starts the server in a goroutine so Bootstrap returns
// promptly. Real listener errors (bind failure, etc.) surface via
// Shutdown when the caller drains the error channel.
func (p *Provider) Boot(ctx context.Context, app provider.Application) error {
	p.errored = make(chan error, 1)

	go func() {
		if err := p.server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			hexlog.Error("web/provider: server exited", "error", err)
			p.errored <- err

			return
		}

		p.errored <- nil
	}()

	return nil
}

// Shutdown drains the server. If Start failed early, returns that
// error; otherwise returns Server.Shutdown's error.
func (p *Provider) Shutdown(ctx context.Context, app provider.Application) error {
	if p.server == nil {
		return nil
	}

	if err := p.server.Shutdown(ctx); err != nil {
		return err
	}

	// Drain any error that Start reported before Shutdown was called.
	select {
	case err := <-p.errored:
		return err
	default:
		return nil
	}
}

// buildOptions reads config into a web.Options and layers ExtraOptions
// on top. Config keys used:
//
//	<ns>.address, <ns>.read_timeout, <ns>.write_timeout,
//	<ns>.idle_timeout, <ns>.health_path, <ns>.ready_path,
//	<ns>.cors (bool), <ns>.disable_request_id (bool),
//	<ns>.disable_recover (bool), <ns>.disable_logger (bool)
func (p *Provider) buildOptions(store *config.Store) web.Options {
	ns := p.Namespace
	if ns == "" {
		ns = "server"
	}

	opts := web.Options{
		Address:          fallback(store.String(ns+".address"), p.ExtraOptions.Address),
		ReadTimeout:      firstDuration(store.Duration(ns+".read_timeout"), p.ExtraOptions.ReadTimeout),
		WriteTimeout:     firstDuration(store.Duration(ns+".write_timeout"), p.ExtraOptions.WriteTimeout),
		IdleTimeout:      firstDuration(store.Duration(ns+".idle_timeout"), p.ExtraOptions.IdleTimeout),
		HealthPath:       fallback(store.String(ns+".health_path"), p.ExtraOptions.HealthPath),
		ReadyPath:        fallback(store.String(ns+".ready_path"), p.ExtraOptions.ReadyPath),
		CORS:             store.Bool(ns+".cors") || p.ExtraOptions.CORS,
		DisableRequestID: store.Bool(ns+".disable_request_id") || p.ExtraOptions.DisableRequestID,
		DisableRecover:   store.Bool(ns+".disable_recover") || p.ExtraOptions.DisableRecover,
		DisableLogger:    store.Bool(ns+".disable_logger") || p.ExtraOptions.DisableLogger,

		// The Configure hook covers structural extras (ReadyFn, custom
		// CORSConfig, custom Transport). These are not string-typed and
		// cannot come from config.
		ReadyFn:    p.ExtraOptions.ReadyFn,
		CORSConfig: p.ExtraOptions.CORSConfig,
	}

	return opts
}

// fallback returns primary if non-empty; otherwise secondary.
func fallback(primary, secondary string) string {
	if primary != "" {
		return primary
	}

	return secondary
}

// firstDuration returns the first non-zero duration.
func firstDuration(primary, secondary time.Duration) time.Duration {
	if primary > 0 {
		return primary
	}

	return secondary
}

// Compile-time confirmation the provider participates in shutdown.
var _ provider.Shutdowner = (*Provider)(nil)
