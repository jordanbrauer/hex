// Package hextest bootstraps a hex.App for tests: env pinned to Test,
// providers registered, Bootstrap run, Shutdown auto-scheduled via
// t.Cleanup.
//
// Typical use from a consumer test:
//
//	func TestSomething(t *testing.T) {
//	    app := hextest.NewApp(t, app.Providers()...)
//	    // ...
//	}
//
// where the consumer's app package exposes a Providers() []provider.Service
// helper that returns the same slice its Boot() function uses. This lets
// tests exercise the real wiring while config env overlays
// (config/<name>.test.toml) redirect drivers to memory/in-process
// implementations.
package hextest

import (
	"context"
	"testing"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/env"
	"github.com/jordanbrauer/hex/provider"
)

// NewApp constructs a fully-booted hex.App with:
//   - Environment pinned to env.Test (redundant with env.Detect's
//     testing.Testing() branch, but explicit for readability)
//   - Every provider passed here registered
//   - Bootstrap run against context.Background
//   - Shutdown scheduled via t.Cleanup
//
// If Register or Bootstrap fails, the test fails immediately (via
// t.Fatal). Callers who want to control the ctx or handle errors
// themselves should assemble the app manually.
func NewApp(t *testing.T, providers ...provider.Service) *hex.App {
	t.Helper()

	app := hex.New(hex.WithEnvironment(env.Test))

	if err := app.Register(providers...); err != nil {
		t.Fatalf("hextest: register: %v", err)
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("hextest: bootstrap: %v", err)
	}

	t.Cleanup(func() {
		if err := app.Shutdown(context.Background()); err != nil {
			t.Errorf("hextest: shutdown: %v", err)
		}
	})

	return app
}
