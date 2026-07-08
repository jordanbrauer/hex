package hextest_test

import (
	"context"
	"testing"

	"github.com/jordanbrauer/hex/env"
	"github.com/jordanbrauer/hex/hextest"
	"github.com/jordanbrauer/hex/provider"
)

// spyProvider records which lifecycle hooks ran.
type spyProvider struct {
	provider.Base
	registered bool
	shutdown   bool
	seenEnv    env.Environment
}

func (p *spyProvider) Register(app provider.Application) error {
	p.registered = true
	p.seenEnv = app.Environment()

	return nil
}

func (p *spyProvider) Shutdown(context.Context, provider.Application) error {
	p.shutdown = true

	return nil
}

func TestNewApp_pinnedTestEnvAndProviderRuns(t *testing.T) {
	spy := &spyProvider{}

	app := hextest.NewApp(t, spy)

	if !spy.registered {
		t.Errorf("provider.Register was not called")
	}

	if got := spy.seenEnv; got != env.Test {
		t.Errorf("provider saw env %v, want Test", got)
	}

	if got := app.Environment(); got != env.Test {
		t.Errorf("app.Environment() = %v, want Test", got)
	}
}

func TestNewApp_shutdownRunsOnCleanup(t *testing.T) {
	spy := &spyProvider{}

	// Nested test scope so t.Cleanup fires when the subtest ends,
	// giving the outer test a chance to observe the spy.
	t.Run("inner", func(t *testing.T) {
		hextest.NewApp(t, spy)
	})

	if !spy.shutdown {
		t.Errorf("provider.Shutdown was not called after subtest cleanup")
	}
}
