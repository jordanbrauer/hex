// Package provider is the default hex/config service provider.
//
// It loads TOML + CUE files from a caller-supplied set of Sources at
// Register time, installs the resulting Store as hex/config's package-
// level default, and binds the *Store into the container under
// "config" so downstream providers can resolve it via
// container.Make[*config.Store](app, "config").
package provider

import (
	"io/fs"

	"github.com/jordanbrauer/hex/config"
	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/provider"
)

// Provider loads hex/config and installs it as the package-level default.
// Register this before any other provider that reads config values.
type Provider struct {
	provider.Base

	// Sources is the ordered layer stack. Framework providers contribute
	// their own defaults via their exported Configs() fs.FS; the
	// application's own config directory is typically added last so it
	// overrides framework defaults.
	Sources []fs.FS

	// SourcesDir is the subdirectory scanned within each Source. Empty
	// means the source's root.
	SourcesDir string

	// UserDir is an optional on-disk directory holding per-namespace
	// override files. Missing UserDir is not an error.
	UserDir string

	// EnvMap / EnvMapFile / EnvFile: env-var binding config. Optional.
	EnvMap     fs.FS
	EnvMapFile string
	EnvFile    string

	// StrictValidation, when true, requires a CUE schema for every
	// loaded namespace.
	StrictValidation bool

	store *config.Store
}

// Register loads config and installs it as the package-level default.
// It also binds *config.Store into the container under "config".
//
// The Config passed to config.Load picks up the app's Environment so
// files matching <ns>.<env>.toml overlay their base counterparts.
func (p *Provider) Register(app provider.Application) error {
	cfg := config.Config{
		Sources:          p.Sources,
		SourcesDir:       p.SourcesDir,
		UserDir:          p.UserDir,
		EnvMap:           p.EnvMap,
		EnvMapFile:       p.EnvMapFile,
		EnvFile:          p.EnvFile,
		Environment:      string(app.Environment()),
		StrictValidation: p.StrictValidation,
	}

	store, err := config.Load(cfg)
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
