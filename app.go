// Package hex is a Go application framework: an IoC container, service
// providers, event bus, and bootstrap orchestration in one opinionated
// module.
//
// A typical program creates an *App, registers providers, calls
// Bootstrap to run their lifecycle hooks in order, does its work, and
// finally calls Shutdown to release resources in reverse order:
//
//	app := hex.New()
//	app.Register(&provider.Database{}, &provider.HTTP{})
//	if err := app.Bootstrap(ctx); err != nil { log.Fatal(err) }
//	defer app.Shutdown(ctx)
//
// The subpackages (container, events, provider) are usable on their own,
// but App wires them together and satisfies provider.Application so
// providers can bind and resolve dependencies through it.
package hex

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/events"
	"github.com/jordanbrauer/hex/provider"
)

// App is the process-wide application kernel. It owns the IoC container, the
// provider registry, and the event bus, and exposes the surface providers
// interact with. Zero-value App is not usable; call New.
type App struct {
	container *container.Container
	events    *events.Bus
	providers *provider.Registry

	mu       sync.Mutex
	booted   bool
	bootedAt time.Time
}

// Option configures a new App. Pass options to New.
type Option func(*App)

// WithContainer replaces the default container. Useful for tests that need to
// pre-seed bindings.
func WithContainer(c *container.Container) Option {
	return func(a *App) { a.container = c }
}

// WithEventBus replaces the default event bus. Useful for tests or for
// consumers that need to share a bus across multiple Apps.
func WithEventBus(b *events.Bus) Option {
	return func(a *App) { a.events = b }
}

// New returns a fresh App with an empty container, event bus, and provider
// registry. Options override the defaults.
func New(opts ...Option) *App {
	a := &App{
		container: container.New(),
		events:    events.New(),
		providers: provider.NewRegistry(),
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Register appends providers to the registry. They will be registered and
// booted in the order supplied across all Register calls. Register may only
// be called before Bootstrap; calling it after returns an error.
func (a *App) Register(providers ...provider.Service) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.booted {
		return errors.New("hex: cannot Register providers after Bootstrap")
	}

	a.providers.Add(providers...)

	return nil
}

// MustRegister is like Register but panics on error. Convenient in main and
// setup code where a Register failure means the program cannot proceed.
func (a *App) MustRegister(providers ...provider.Service) {
	if err := a.Register(providers...); err != nil {
		panic(err)
	}
}

// Bootstrap runs the two-phase provider lifecycle: every provider's Register
// runs in insertion order, then every provider's Boot runs in the same order.
// Bootstrap is idempotent — a second call returns nil without re-invoking
// hooks — so consumers can call it from a defensive main safely.
func (a *App) Bootstrap(ctx context.Context) error {
	a.mu.Lock()
	if a.booted {
		a.mu.Unlock()

		return nil
	}
	a.mu.Unlock()

	if err := a.providers.Register(a); err != nil {
		return err
	}

	if err := a.providers.Boot(ctx, a); err != nil {
		return err
	}

	a.mu.Lock()
	a.booted = true
	a.bootedAt = time.Now()
	a.mu.Unlock()

	return nil
}

// Shutdown runs Shutdown on every booted provider that implements
// provider.Shutdowner, in reverse boot order. It returns a joined error if
// any provider fails, but always visits every provider. Shutdown is a no-op
// if Bootstrap has not run.
func (a *App) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	if !a.booted {
		a.mu.Unlock()

		return nil
	}

	a.booted = false
	a.mu.Unlock()

	return a.providers.Shutdown(ctx, a)
}

// BootedAt returns the time Bootstrap completed. Returns the zero Time if
// Bootstrap has not run.
func (a *App) BootedAt() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.bootedAt
}

// Container returns the underlying IoC container. Prefer Bind, Singleton,
// and Make on App itself; use Container only when you need methods the App
// does not delegate (such as List or Count).
func (a *App) Container() *container.Container { return a.container }

// Events returns the underlying event bus. Prefer On and Emit on App
// itself; use Events for less common operations like EmitAsync.
func (a *App) Events() *events.Bus { return a.events }

// --- provider.Application surface ------------------------------------------

// Bind delegates to the underlying container.
func (a *App) Bind(name string, fn container.Factory) { a.container.Bind(name, fn) }

// Singleton delegates to the underlying container.
func (a *App) Singleton(name string, fn container.Factory) { a.container.Singleton(name, fn) }

// Make delegates to the underlying container.
func (a *App) Make(name string) (any, error) { return a.container.Make(name) }

// On delegates to the underlying event bus.
func (a *App) On(event string, fn events.Subscriber) func() { return a.events.On(event, fn) }

// Emit delegates to the underlying event bus.
func (a *App) Emit(event string, data ...any) error { return a.events.Emit(event, data...) }

// Compile-time proof that *App satisfies provider.Application. If a provider
// change breaks this, the build fails here before anywhere else.
var _ provider.Application = (*App)(nil)
