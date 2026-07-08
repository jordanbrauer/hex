package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/env"
	"github.com/jordanbrauer/hex/events"
	"github.com/jordanbrauer/hex/provider"
)

// fakeApp is a minimal provider.Application used by registry tests. It backs
// bindings with a real container and events with a real bus so lifecycle
// hooks can exercise both surfaces.
type fakeApp struct {
	c *container.Container
	b *events.Bus
}

func newFakeApp() *fakeApp {
	return &fakeApp{c: container.New(), b: events.New()}
}

func (a *fakeApp) Bind(name string, fn container.Factory)      { a.c.Bind(name, fn) }
func (a *fakeApp) Singleton(name string, fn container.Factory) { a.c.Singleton(name, fn) }
func (a *fakeApp) Make(name string) (any, error)               { return a.c.Make(name) }
func (a *fakeApp) On(event string, fn events.Subscriber) func() {
	return a.b.On(event, fn)
}
func (a *fakeApp) Emit(event string, data ...any) error { return a.b.Emit(event, data...) }
func (a *fakeApp) Environment() env.Environment         { return env.Test }

// recorder tracks lifecycle calls for assertion.
type recorder struct {
	name     string
	log      *[]string
	regErr   error
	bootErr  error
	closeErr error
}

func (r *recorder) Register(provider.Application) error {
	*r.log = append(*r.log, r.name+":register")

	return r.regErr
}

func (r *recorder) Boot(context.Context, provider.Application) error {
	*r.log = append(*r.log, r.name+":boot")

	return r.bootErr
}

func (r *recorder) Shutdown(context.Context, provider.Application) error {
	*r.log = append(*r.log, r.name+":shutdown")

	return r.closeErr
}

// bootOnly implements Service but not Shutdowner so we can prove the registry
// skips providers that do not opt into shutdown.
type bootOnly struct {
	name string
	log  *[]string
}

func (p *bootOnly) Register(provider.Application) error {
	*p.log = append(*p.log, p.name+":register")

	return nil
}

func (p *bootOnly) Boot(context.Context, provider.Application) error {
	*p.log = append(*p.log, p.name+":boot")

	return nil
}

func TestRegistry_registerBootShutdownOrder(t *testing.T) {
	log := []string{}
	r := provider.NewRegistry()
	r.Add(
		&recorder{name: "a", log: &log},
		&recorder{name: "b", log: &log},
		&recorder{name: "c", log: &log},
	)

	app := newFakeApp()

	if err := r.Register(app); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	if err := r.Boot(context.Background(), app); err != nil {
		t.Fatalf("Boot error = %v", err)
	}

	if err := r.Shutdown(context.Background(), app); err != nil {
		t.Fatalf("Shutdown error = %v", err)
	}

	want := []string{
		"a:register", "b:register", "c:register",
		"a:boot", "b:boot", "c:boot",
		"c:shutdown", "b:shutdown", "a:shutdown",
	}

	if len(log) != len(want) {
		t.Fatalf("call log = %v, want %v", log, want)
	}

	for i := range want {
		if log[i] != want[i] {
			t.Errorf("log[%d] = %q, want %q", i, log[i], want[i])
		}
	}
}

func TestRegistry_registerErrorStopsSequence(t *testing.T) {
	log := []string{}
	r := provider.NewRegistry()
	r.Add(
		&recorder{name: "a", log: &log},
		&recorder{name: "b", log: &log, regErr: errors.New("nope")},
		&recorder{name: "c", log: &log},
	)

	err := r.Register(newFakeApp())
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	// c must not be registered.
	if len(log) != 2 {
		t.Errorf("log = %v, want 2 entries (a, b)", log)
	}
}

func TestRegistry_bootErrorStopsSequenceButShutsDownWhatBooted(t *testing.T) {
	log := []string{}
	r := provider.NewRegistry()
	r.Add(
		&recorder{name: "a", log: &log},
		&recorder{name: "b", log: &log, bootErr: errors.New("boom")},
		&recorder{name: "c", log: &log},
	)

	app := newFakeApp()

	if err := r.Register(app); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	if err := r.Boot(context.Background(), app); err == nil {
		t.Fatal("Boot returned nil error")
	}

	if err := r.Shutdown(context.Background(), app); err != nil {
		t.Fatalf("Shutdown error = %v", err)
	}

	// a booted, b failed, c never ran. Only a should be shut down.
	want := []string{
		"a:register", "b:register", "c:register",
		"a:boot", "b:boot",
		"a:shutdown",
	}

	if len(log) != len(want) {
		t.Fatalf("log = %v, want %v", log, want)
	}

	for i := range want {
		if log[i] != want[i] {
			t.Errorf("log[%d] = %q, want %q", i, log[i], want[i])
		}
	}
}

func TestRegistry_shutdownSkipsNonShutdowners(t *testing.T) {
	log := []string{}
	r := provider.NewRegistry()
	r.Add(
		&recorder{name: "a", log: &log},
		&bootOnly{name: "b", log: &log},
		&recorder{name: "c", log: &log},
	)

	app := newFakeApp()
	_ = r.Register(app)
	_ = r.Boot(context.Background(), app)

	if err := r.Shutdown(context.Background(), app); err != nil {
		t.Fatalf("Shutdown error = %v", err)
	}

	// b:shutdown must not appear.
	for _, entry := range log {
		if entry == "b:shutdown" {
			t.Errorf("bootOnly provider was shut down: %v", log)
		}
	}
}

func TestRegistry_shutdownJoinsErrors(t *testing.T) {
	log := []string{}
	e1 := errors.New("first-fail")
	e2 := errors.New("second-fail")

	r := provider.NewRegistry()
	r.Add(
		&recorder{name: "a", log: &log, closeErr: e1},
		&recorder{name: "b", log: &log, closeErr: e2},
	)

	app := newFakeApp()
	_ = r.Register(app)
	_ = r.Boot(context.Background(), app)

	err := r.Shutdown(context.Background(), app)
	if err == nil {
		t.Fatal("Shutdown returned nil error")
	}

	if !errors.Is(err, e1) || !errors.Is(err, e2) {
		t.Errorf("Shutdown error = %v, want both %v and %v", err, e1, e2)
	}
}

func TestRegistry_shutdownIsIdempotent(t *testing.T) {
	log := []string{}
	r := provider.NewRegistry()
	r.Add(&recorder{name: "a", log: &log})

	app := newFakeApp()
	_ = r.Register(app)
	_ = r.Boot(context.Background(), app)

	_ = r.Shutdown(context.Background(), app)
	_ = r.Shutdown(context.Background(), app)

	shutdowns := 0
	for _, entry := range log {
		if entry == "a:shutdown" {
			shutdowns++
		}
	}

	if shutdowns != 1 {
		t.Errorf("provider shut down %d times, want 1", shutdowns)
	}
}

func TestBase_isNoOpService(t *testing.T) {
	// Base must satisfy Service without any code from the embedder.
	type mine struct{ provider.Base }

	var s provider.Service = &mine{}

	if err := s.Register(nil); err != nil {
		t.Errorf("Base.Register error = %v, want nil", err)
	}

	if err := s.Boot(context.Background(), nil); err != nil {
		t.Errorf("Base.Boot error = %v, want nil", err)
	}

	// And it must NOT satisfy Shutdowner by default (ADR: opt-in shutdown).
	if _, ok := s.(provider.Shutdowner); ok {
		t.Errorf("Base unexpectedly satisfies Shutdowner")
	}
}
