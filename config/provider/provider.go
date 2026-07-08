// Package provider is the default hex/config service provider.
//
// It loads embedded defaults + user override files + env.yaml bindings at
// Register time, installs the resulting Store as hex/config's package-
// level default, and binds the *Store into the container under
// "config".
//
// Consumers typically wire this from a small factory in
// app/provider/config.go rather than instantiating directly, so hooks
// and paths stay visible in the consumer's repo.
package provider

import (
	"embed"

	"github.com/jordanbrauer/hex/config"
	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/provider"
)

// Provider loads hex/config and installs it as the package-level default.
// Register this before any other provider that reads config values via
// config.String / config.Int / etc.
type Provider struct {
	provider.Base

	// Defaults is the embed.FS containing baseline TOML files (one per
	// namespace). Required.
	Defaults embed.FS

	// DefaultsDir is the subdirectory within Defaults that holds *.toml.
	// Empty means the FS root.
	DefaultsDir string

	// UserDir is an optional on-disk directory holding per-namespace
	// override files. Missing UserDir is not an error.
	UserDir string

	// EnvMap is an embed.FS containing the env-var binding YAML.
	// Optional.
	EnvMap embed.FS

	// EnvMapFile is the path within EnvMap to the binding YAML. Empty
	// means no env bindings.
	EnvMapFile string

	// EnvFile is an optional .env file loaded before env-var bindings
	// resolve. Missing files are ignored.
	EnvFile string

	// store is the loaded Store; populated during Register.
	store *config.Store
}

// Register loads config and installs it as the package-level default.
// It also binds *config.Store into the container under "config".
func (p *Provider) Register(app provider.Application) error {
	store, err := config.Load(config.Config{
		Defaults:    p.Defaults,
		DefaultsDir: p.DefaultsDir,
		UserDir:     p.UserDir,
		EnvMap:      p.EnvMap,
		EnvMapFile:  p.EnvMapFile,
		EnvFile:     p.EnvFile,
	})
	if err != nil {
		return err
	}

	p.store = store
	config.SetDefault(store)

	app.Singleton("config", func(*container.Container) (any, error) {
		return p.store, nil
	})

	return nil
}
