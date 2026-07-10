// Package app wires this application's service providers into the hex
// kernel. Providers themselves live in the app/provider subpackage;
// this file lists them in Boot order.
//
// This app has no file-based config (no hex/config provider), so
// there's nothing to order Lua against — it's the only provider.
package app

import (
	"context"
	"fmt"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/log"

	"github.com/jordanbrauer/hex/examples/bare-hex-app/app/provider"
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
