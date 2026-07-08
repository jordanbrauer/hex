package hex_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/provider"
)

type recordingProvider struct {
	name string
	log  *[]string

	regFn  func(provider.Application) error
	bootFn func(context.Context, provider.Application) error
	closer func(context.Context, provider.Application) error
}

func (p *recordingProvider) Register(app provider.Application) error {
	*p.log = append(*p.log, p.name+":register")

	if p.regFn != nil {
		return p.regFn(app)
	}

	return nil
}

func (p *recordingProvider) Boot(ctx context.Context, app provider.Application) error {
	*p.log = append(*p.log, p.name+":boot")

	if p.bootFn != nil {
		return p.bootFn(ctx, app)
	}

	return nil
}

func (p *recordingProvider) Shutdown(ctx context.Context, app provider.Application) error {
	*p.log = append(*p.log, p.name+":shutdown")

	if p.closer != nil {
		return p.closer(ctx, app)
	}

	return nil
}

func TestApp_bootstrapRunsRegisterThenBootInOrder(t *testing.T) {
	log := []string{}
	app := hex.New()

	if err := app.Register(
		&recordingProvider{name: "a", log: &log},
		&recordingProvider{name: "b", log: &log},
	); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap error = %v", err)
	}

	want := []string{"a:register", "b:register", "a:boot", "b:boot"}
	if len(log) != len(want) {
		t.Fatalf("log = %v, want %v", log, want)
	}

	for i := range want {
		if log[i] != want[i] {
			t.Errorf("log[%d] = %q, want %q", i, log[i], want[i])
		}
	}
}

func TestApp_bootstrapIsIdempotent(t *testing.T) {
	log := []string{}
	app := hex.New()

	if err := app.Register(&recordingProvider{name: "a", log: &log}); err != nil {
		t.Fatal(err)
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("first Bootstrap error = %v", err)
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("second Bootstrap error = %v", err)
	}

	// Register + Boot should each appear exactly once.
	regs, boots := 0, 0
	for _, e := range log {
		switch e {
		case "a:register":
			regs++
		case "a:boot":
			boots++
		}
	}

	if regs != 1 || boots != 1 {
		t.Errorf("register=%d boot=%d, want 1 each; log=%v", regs, boots, log)
	}
}

func TestApp_registerAfterBootstrapFails(t *testing.T) {
	app := hex.New()

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap error = %v", err)
	}

	err := app.Register(&recordingProvider{name: "late", log: new([]string)})
	if err == nil {
		t.Errorf("Register after Bootstrap returned nil error")
	}
}

func TestApp_bootstrapPropagatesRegisterError(t *testing.T) {
	sentinel := errors.New("register fail")
	log := []string{}

	app := hex.New()
	_ = app.Register(&recordingProvider{
		name:  "a",
		log:   &log,
		regFn: func(provider.Application) error { return sentinel },
	})

	err := app.Bootstrap(context.Background())
	if !errors.Is(err, sentinel) {
		t.Errorf("Bootstrap error = %v, want wraps %v", err, sentinel)
	}
}

func TestApp_bootstrapPropagatesBootError(t *testing.T) {
	sentinel := errors.New("boot fail")
	log := []string{}

	app := hex.New()
	_ = app.Register(&recordingProvider{
		name:   "a",
		log:    &log,
		bootFn: func(context.Context, provider.Application) error { return sentinel },
	})

	err := app.Bootstrap(context.Background())
	if !errors.Is(err, sentinel) {
		t.Errorf("Bootstrap error = %v, want wraps %v", err, sentinel)
	}
}

func TestApp_shutdownReverseOrder(t *testing.T) {
	log := []string{}

	app := hex.New()
	_ = app.Register(
		&recordingProvider{name: "a", log: &log},
		&recordingProvider{name: "b", log: &log},
		&recordingProvider{name: "c", log: &log},
	)

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown error = %v", err)
	}

	// Filter for :shutdown entries.
	var order []string
	for _, e := range log {
		if len(e) > 9 && e[len(e)-9:] == ":shutdown" {
			order = append(order, e)
		}
	}

	want := []string{"c:shutdown", "b:shutdown", "a:shutdown"}
	if len(order) != len(want) {
		t.Fatalf("shutdown order = %v, want %v", order, want)
	}

	for i := range want {
		if order[i] != want[i] {
			t.Errorf("shutdown[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestApp_shutdownBeforeBootstrapIsNoop(t *testing.T) {
	if err := hex.New().Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown before Bootstrap error = %v, want nil", err)
	}
}

func TestApp_bootedAtSetAfterBootstrap(t *testing.T) {
	app := hex.New()

	if !app.BootedAt().IsZero() {
		t.Errorf("BootedAt before Bootstrap is not zero: %v", app.BootedAt())
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}

	if app.BootedAt().IsZero() {
		t.Errorf("BootedAt after Bootstrap is zero")
	}
}

func TestApp_satisfiesProviderApplication_bindResolve(t *testing.T) {
	app := hex.New()

	app.Singleton("greeting", func(*container.Container) (any, error) {
		return "hello", nil
	})

	got, err := container.Make[string](app.Container(), "greeting")
	if err != nil {
		t.Fatalf("Make error = %v", err)
	}

	if got != "hello" {
		t.Errorf("Make = %q, want %q", got, "hello")
	}
}

func TestApp_eventDelegation(t *testing.T) {
	app := hex.New()

	received := ""
	off := app.On("hello", func(data ...any) error {
		if len(data) > 0 {
			received, _ = data[0].(string)
		}

		return nil
	})
	defer off()

	if err := app.Emit("hello", "world"); err != nil {
		t.Fatalf("Emit error = %v", err)
	}

	if received != "world" {
		t.Errorf("subscriber saw %q, want %q", received, "world")
	}
}

func TestApp_providersCanResolveEachOther(t *testing.T) {
	// A resolves in Boot the binding B registered in Register — proves that
	// bindings from all Register phases are visible during Boot.
	log := []string{}
	app := hex.New()

	_ = app.Register(
		&recordingProvider{
			name: "b",
			log:  &log,
			regFn: func(a provider.Application) error {
				a.Singleton("token", func(*container.Container) (any, error) {
					return "abc123", nil
				})

				return nil
			},
		},
		&recordingProvider{
			name: "a",
			log:  &log,
			bootFn: func(_ context.Context, a provider.Application) error {
				v, err := a.Make("token")
				if err != nil {
					return err
				}

				if v.(string) != "abc123" {
					t.Errorf("resolved token %v, want abc123", v)
				}

				return nil
			},
		},
	)

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap error = %v", err)
	}
}
