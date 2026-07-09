package provider

import (
	"io/fs"

	"github.com/jordanbrauer/hex/config/provider"
	dbprovider "github.com/jordanbrauer/hex/db/provider"
	logprovider "github.com/jordanbrauer/hex/log/provider"
	luaprovider "github.com/jordanbrauer/hex/lua/provider"
	webprovider "github.com/jordanbrauer/hex/web/provider"

	"github.com/jordanbrauer/hex/examples/swapi/config"
)

// Config wires the app's configuration.
//
// Sources layer earliest-to-latest: framework defaults (from each
// enabled hex/<pkg>/provider's Configs()) first, then the app's own
// config/ directory last so the app can override anything.
//
// Customise by editing this factory: change the env.yaml path, the
// user override directory, or add third-party package Configs() to
// the Sources list.
func Config() *provider.Provider {
	return &provider.Provider{
		Sources: []fs.FS{
			logprovider.Configs(),
			luaprovider.Configs(),
			dbprovider.Configs(),
			webprovider.Configs(),
			config.Files,
		},

		EnvMap:     config.Files,
		EnvMapFile: config.EnvMapFile,

		// UserDir: "~/.config/swapi", // uncomment to layer user overrides
		// EnvFile: ".env",                       // uncomment to load a .env file
	}
}
