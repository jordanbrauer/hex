// Package provider is the default hex/log service provider.
//
// It reads the log configuration (level + optional caller / timestamp
// toggles) from hex/config and applies it via hex/log.SetLevel etc. The
// provider assumes hex/log.Init has already been called by main() —
// this provider only adjusts levels/flags after config is loaded.
package provider

import (
	"github.com/jordanbrauer/hex/config"
	hexlog "github.com/jordanbrauer/hex/log"
	"github.com/jordanbrauer/hex/provider"
)

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
	ns := p.Namespace
	if ns == "" {
		ns = "log"
	}

	level := p.DefaultLevel
	if level == 0 {
		level = hexlog.InfoLevel
	}

	if raw := config.String(ns + ".level"); raw != "" {
		if parsed, err := hexlog.ParseLevel(raw); err == nil {
			level = parsed
		} else {
			hexlog.Warn("log/provider: ignoring invalid level", "value", raw, "error", err)
		}
	}

	hexlog.SetLevel(level)

	if config.Has(ns + ".caller") {
		hexlog.Init(
			hexlog.WithLevel(level),
			hexlog.WithCaller(config.Bool(ns+".caller")),
			hexlog.WithTimestamp(config.Bool(ns+".timestamp")),
		)
	}

	return nil
}
