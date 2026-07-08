// Package provider is the default hex/log service provider.
//
// It reads the log configuration (level + optional caller / timestamp
// toggles) from hex/config and applies it via hex/log.SetLevel etc. The
// provider assumes hex/log.Init has already been called by main() —
// this provider only adjusts levels/flags after config is loaded.
//
// Configs returns the embedded framework defaults + CUE schema for the
// "log" namespace. Consumer factories add Configs() to their
// hex/config Provider.Sources so log defaults are always present.
package provider

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/jordanbrauer/hex/config"
	"github.com/jordanbrauer/hex/container"
	hexlog "github.com/jordanbrauer/hex/log"
	"github.com/jordanbrauer/hex/provider"
)

//go:embed config
var configFS embed.FS

// Configs returns the embedded default TOML + CUE files this provider
// contributes to hex/config. Consumer factories add it to
// hex/config.Provider.Sources.
func Configs() fs.FS {
	sub, err := fs.Sub(configFS, "config")
	if err != nil {
		// Impossible: the config dir is bundled at build time.
		panic("log/provider: embedded config subdir missing: " + err.Error())
	}

	return sub
}

// Provider reads log config values and applies them to hex/log.
type Provider struct {
	provider.Base

	// Namespace is the config namespace read for log settings.
	// Defaults to "log".
	Namespace string

	// DefaultLevel is applied when the config key is unset or invalid.
	// Zero means InfoLevel.
	DefaultLevel hexlog.Level
}

// Register applies configured log settings. Runs at Register (not Boot)
// so downstream providers see the correct log level during their own
// Register/Boot phases.
func (p *Provider) Register(app provider.Application) error {
	store, err := container.Make[*config.Store](app, "config")
	if err != nil {
		return fmt.Errorf("log/provider: resolve config: %w", err)
	}

	ns := p.Namespace
	if ns == "" {
		ns = "log"
	}

	level := p.DefaultLevel
	if level == 0 {
		level = hexlog.InfoLevel
	}

	if raw := store.String(ns + ".level"); raw != "" {
		if parsed, err := hexlog.ParseLevel(raw); err == nil {
			level = parsed
		} else {
			hexlog.Warn("log/provider: ignoring invalid level", "value", raw, "error", err)
		}
	}

	hexlog.SetLevel(level)

	if store.Has(ns + ".caller") {
		hexlog.Init(
			hexlog.WithLevel(level),
			hexlog.WithCaller(store.Bool(ns+".caller")),
			hexlog.WithTimestamp(store.Bool(ns+".timestamp")),
		)
	}

	return nil
}
