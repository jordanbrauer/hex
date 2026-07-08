// Package provider is the default hex/cron service provider.
//
// It constructs a Cron scheduler at Register time, binds it into the
// container under "scheduler", starts it during Boot, and stops it
// gracefully during Shutdown.
//
// Register your jobs from another provider that resolves "scheduler"
// out of the container during its own Boot.
package provider

import (
	"context"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/cron"
	"github.com/jordanbrauer/hex/provider"
)

// Provider wires a *cron.Cron scheduler into the container.
type Provider struct {
	provider.Base

	// Binding is the container name. Defaults to "scheduler".
	Binding string

	// Options is passed to cron.New (e.g. cron.WithSeconds()).
	Options []cron.Option

	// ShutdownTimeout caps the wait for in-flight jobs on Shutdown.
	// Zero means "use the context passed to Shutdown as-is."
	//
	// Not currently used to add its own timeout because provider
	// consumers already own the shutdown ctx — keeping it as a field
	// leaves the door open.
	sched *cron.Cron
}

// Register constructs the scheduler and binds it.
func (p *Provider) Register(app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "scheduler"
	}

	p.sched = cron.New(p.Options...)

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.sched, nil
	})

	return nil
}

// Boot starts the scheduler. Jobs registered before Boot run once Boot
// returns; jobs registered after Boot pick up on their next tick.
func (p *Provider) Boot(ctx context.Context, app provider.Application) error {
	p.sched.Start()

	return nil
}

// Shutdown stops the scheduler and waits for in-flight jobs or ctx.
func (p *Provider) Shutdown(ctx context.Context, app provider.Application) error {
	if p.sched == nil {
		return nil
	}

	return p.sched.Stop(ctx)
}

// Compile-time confirmation the provider participates in shutdown.
var _ provider.Shutdowner = (*Provider)(nil)
