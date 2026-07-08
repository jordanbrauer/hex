// Package provider defines the service provider contract and the ordered
// registry that drives application bootstrap.
//
// A Service registers bindings during Register, then performs any startup
// work in Boot. Providers that need cleanup implement the optional
// Shutdowner interface; the Registry only calls Shutdown on providers that
// declare it. Boot order matches registration order; shutdown order is the
// reverse.
package provider

import (
	"context"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/env"
	"github.com/jordanbrauer/hex/events"
)

// Application is the surface a provider sees during Register and Boot. It is
// intentionally narrow: providers can register bindings, resolve previously
// registered ones, and publish or subscribe to events. This lets the
// Registry drive lifecycle without exposing the full app kernel to
// providers.
type Application interface {
	Bind(name string, fn container.Factory)
	Singleton(name string, fn container.Factory)
	Make(name string) (any, error)

	On(event string, fn events.Subscriber) func()
	Emit(event string, data ...any) error

	// Environment reports the runtime environment the app is running
	// in. Providers can consult this to swap driver defaults (memory
	// backends in tests, real services in production) without waiting
	// on config layering.
	Environment() env.Environment
}

// Service is a service provider. Register runs first for every provider in
// registration order (all Register calls complete before any Boot call).
// Boot runs after all providers are registered, again in registration
// order. Any error from either phase aborts bootstrap.
type Service interface {
	Register(app Application) error
	Boot(ctx context.Context, app Application) error
}

// Shutdowner is the optional interface implemented by providers that need to
// release resources on application shutdown. Providers that do not need
// cleanup should not implement it — the Registry skips them entirely.
type Shutdowner interface {
	Shutdown(ctx context.Context, app Application) error
}

// Base is an embeddable no-op Service. Embed it in concrete providers to
// satisfy the Service interface while only implementing the methods that
// matter. Base does not implement Shutdowner — add a Shutdown method to
// your provider if it needs cleanup.
type Base struct{}

// Register is a no-op default. Override in concrete providers.
func (Base) Register(Application) error { return nil }

// Boot is a no-op default. Override in concrete providers.
func (Base) Boot(context.Context, Application) error { return nil }
