// Package provider is the default hex/queue service provider.
//
// v1 supports the in-process memory backend out of the box. Consumers
// wanting sqlite/redis/sqs/etc. supply a Backend function that
// constructs the queue.Queue implementation of their choice — the
// framework does not import driver packages here to keep the
// dependency footprint minimal (matches ADR-0004's pattern for db).
package provider

import (
	"context"

	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/provider"
	"github.com/jordanbrauer/hex/queue"
	"github.com/jordanbrauer/hex/queue/memory"
)

// Provider wires a queue.Queue into the container.
type Provider struct {
	provider.Base

	// Binding is the container name. Defaults to "queue".
	Binding string

	// Backend, when set, returns the concrete queue implementation.
	// When nil, the memory backend is used (fine for tests and
	// single-process deployments; not durable).
	Backend func() queue.Queue

	q queue.Queue
}

// Register constructs the queue and binds it.
func (p *Provider) Register(app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "queue"
	}

	if p.Backend != nil {
		p.q = p.Backend()
	} else {
		p.q = memory.New(memory.Options{})
	}

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.q, nil
	})

	return nil
}

// Shutdown closes the queue (drains subscribers, waits for in-flight
// handlers up to ctx).
func (p *Provider) Shutdown(ctx context.Context, app provider.Application) error {
	if p.q == nil {
		return nil
	}

	return p.q.Close(ctx)
}

// Compile-time confirmation the provider participates in shutdown.
var _ provider.Shutdowner = (*Provider)(nil)
