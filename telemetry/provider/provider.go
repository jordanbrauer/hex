// Package provider is the default hex/telemetry service provider.
//
// It reads telemetry config and calls telemetry.Setup during Boot,
// installing global tracer/meter providers. Shutdown flushes exporters.
package provider

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	hexbuild "github.com/jordanbrauer/hex/build"
	"github.com/jordanbrauer/hex/config"
	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/provider"
	"github.com/jordanbrauer/hex/telemetry"
)

//go:embed config
var configFS embed.FS

// Configs returns the embedded default TOML + CUE files this provider
// contributes to hex/config. Add it to hex/config.Provider.Sources.
func Configs() fs.FS {
	sub, err := fs.Sub(configFS, "config")
	if err != nil {
		panic("telemetry/provider: embedded config subdir missing: " + err.Error())
	}

	return sub
}

// Provider wires OpenTelemetry into the app.
type Provider struct {
	provider.Base

	// Binding is the container name for the telemetry Provider.
	// Defaults to "telemetry".
	Binding string

	// Namespace is the config namespace read for telemetry settings.
	// Defaults to "telemetry".
	Namespace string

	// ServiceName overrides config's service_name. Consumers typically
	// leave this empty and set it via config.
	ServiceName string

	// ExporterOverride replaces the config-selected exporter.
	// Zero value means "use config".
	ExporterOverride telemetry.Exporter

	// AdditionalAttrs merges with config-declared resource attributes.
	AdditionalAttrs map[string]string

	tp *telemetry.Provider
}

// Boot configures OTel per config, installs globals, binds the
// Provider into the container.
func (p *Provider) Boot(ctx context.Context, app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "telemetry"
	}

	store, err := container.Make[*config.Store](app, "config")
	if err != nil {
		return fmt.Errorf("telemetry/provider: resolve config: %w", err)
	}

	opts := p.buildOptions(store)

	tp, err := telemetry.Setup(ctx, opts)
	if err != nil {
		return err
	}

	p.tp = tp
	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.tp, nil
	})

	return nil
}

// Shutdown flushes and closes tracer/meter providers.
func (p *Provider) Shutdown(ctx context.Context, app provider.Application) error {
	if p.tp == nil {
		return nil
	}

	return p.tp.Shutdown(ctx)
}

func (p *Provider) buildOptions(store *config.Store) telemetry.Options {
	ns := p.Namespace
	if ns == "" {
		ns = "telemetry"
	}

	name := p.ServiceName
	if name == "" {
		name = store.String(ns + ".service_name")
	}

	// Version defaults to hex/build's Version so telemetry is auto-tagged
	// with the build that emitted it. Consumers can override via config.
	version := store.String(ns + ".service_version")
	if version == "" {
		version = hexbuild.Version()
	}

	env := store.String(ns + ".environment")

	exporter := p.ExporterOverride
	if exporter == 0 {
		exporter = parseExporter(store.String(ns + ".exporter"))
	}

	attrs := make(map[string]string)
	for k, v := range p.AdditionalAttrs {
		attrs[k] = v
	}

	return telemetry.Options{
		ServiceName:        name,
		ServiceVersion:     version,
		Environment:        env,
		Exporter:           exporter,
		ResourceAttributes: attrs,
	}
}

func parseExporter(s string) telemetry.Exporter {
	switch s {
	case "", "stdout":
		return telemetry.ExporterStdout
	case "otlp", "grpc", "otlp-grpc":
		return telemetry.ExporterOTLP
	case "none", "off", "disabled":
		return telemetry.ExporterNone
	default:
		return telemetry.ExporterStdout
	}
}

// Compile-time confirmation the provider participates in shutdown.
var _ provider.Shutdowner = (*Provider)(nil)
