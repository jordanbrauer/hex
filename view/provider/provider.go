// Package provider installs a hex/view Engine as the *web.Server's
// echo.Renderer and binds the engine under "view" in the container.
//
// Register order:
//
//	provider.Web(),   // binds *web.Server under "http"
//	provider.View(),  // this — resolves "http", installs renderer
//
// Any provider that wants to reach the engine directly (rare —
// usually you use c.Render on echo.Context) resolves *view.Engine
// from the container under "view".
package provider

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/provider"
	"github.com/jordanbrauer/hex/view"
	"github.com/jordanbrauer/hex/web"
)

// Provider wires a *view.Engine into the container and installs it as
// the echo.Renderer on the *web.Server bound under WebBinding.
type Provider struct {
	provider.Base

	// Binding is the container key for the *view.Engine. Defaults to
	// "view".
	Binding string

	// WebBinding is the container key for the *web.Server the engine
	// attaches to. Defaults to "http".
	WebBinding string

	// FS is the fs.FS containing template files. Typically an
	// //go:embed of web/views/. Required.
	FS fs.FS

	// Dir is the subdirectory within FS to scan. Empty means the
	// root.
	Dir string

	// Options are passed to view.New verbatim (funcs, custom
	// extension, etc.).
	Options []view.Option

	engine *view.Engine
}

// Register builds the engine and installs it on the server.
func (p *Provider) Register(app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "view"
	}

	webBinding := p.WebBinding
	if webBinding == "" {
		webBinding = "http"
	}

	if p.FS == nil {
		return errors.New("view/provider: FS is nil")
	}

	opts := make([]view.Option, 0, len(p.Options)+1)
	if p.Dir != "" {
		opts = append(opts, view.WithDir(p.Dir))
	}

	opts = append(opts, p.Options...)

	engine, err := view.New(p.FS, opts...)
	if err != nil {
		return fmt.Errorf("view/provider: build engine: %w", err)
	}

	p.engine = engine

	server, err := container.Make[*web.Server](app, webBinding)
	if err != nil {
		return fmt.Errorf("view/provider: resolve %q: %w", webBinding, err)
	}

	server.Echo().Renderer = engine

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.engine, nil
	})

	return nil
}
