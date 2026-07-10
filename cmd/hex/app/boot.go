// Package app wires the hex CLI's own service providers into the hex
// kernel. Providers themselves live in the app/provider subpackage; this
// file lists them in Boot order.
//
// Unlike a scaffolded consumer app, the CLI has no file-based app config
// (it's driven entirely by cobra flags and project-root detection), so
// hex/config and hex/log's providers are not registered here — hex/log is
// initialised directly in main via hexlog.Init(), and --log-level/--env/
// --verbose are handled by hexcli.Root itself.
package app

import (
	"github.com/jordanbrauer/hex"

	"github.com/jordanbrauer/hex/cmd/hex/app/provider"
)

// Boot registers every provider with kernel. Order matters — providers
// are registered and booted in the order they appear here.
//
// hex make:provider inserts new provider registrations above the
// `// hex:providers` marker below. Do not remove the marker.
func Boot(kernel *hex.App) error {
	return kernel.Register(
		provider.Lua(),
		provider.Generator(),
		// hex:providers
	)
}
