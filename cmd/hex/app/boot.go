// Package app wires the hex CLI's own service providers into the hex
// kernel. Providers themselves live in the app/provider subpackage; this
// file lists them in Boot order.
//
// Unlike a scaffolded consumer app, the CLI has no file-based app config
// (it's driven entirely by cobra flags and project-root detection), so
// hex/config's provider is not registered here — --log-level/--env/
// --verbose are handled by cli.Root itself.
package app

import (
	"context"
	"fmt"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/log"

	"github.com/jordanbrauer/hex/cmd/hex/app/provider"
)

// Boot initialises logging and registers every provider with kernel.
// Order matters — providers are registered and booted in the order they
// appear here.
//
// hex make provider inserts new provider registrations above the
// `// hex:providers` marker below. Do not remove the marker.
func Boot(kernel *hex.App) error {
	log.Init()

	return kernel.Register(
		provider.Lua(),
		provider.Generator(),
		// hex:providers
	)
}

// Bootstrap registers providers via Boot, then runs kernel's Register/Boot
// lifecycle. main only has to check one error from one call.
func Bootstrap(ctx context.Context, kernel *hex.App) error {
	if err := Boot(kernel); err != nil {
		return fmt.Errorf("register providers: %w", err)
	}

	if err := kernel.Bootstrap(ctx); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	return nil
}
