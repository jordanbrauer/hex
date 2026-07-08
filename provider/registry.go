package provider

import (
	"context"
	"errors"
	"fmt"
)

// Registry manages an ordered list of service providers and drives their
// lifecycle. The zero value is not usable; call NewRegistry.
type Registry struct {
	providers []Service
	booted    []Service
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Add appends one or more providers to the registry. Order is preserved and
// determines the Register/Boot sequence.
func (r *Registry) Add(providers ...Service) {
	r.providers = append(r.providers, providers...)
}

// Len returns the number of registered providers.
func (r *Registry) Len() int {
	return len(r.providers)
}

// Register invokes Register on every provider in insertion order. It returns
// the first error encountered, aborting the sequence.
func (r *Registry) Register(app Application) error {
	for _, p := range r.providers {
		if err := p.Register(app); err != nil {
			return fmt.Errorf("provider: register %T: %w", p, err)
		}
	}

	return nil
}

// Boot invokes Boot on every provider in insertion order, recording each
// successfully booted provider so Shutdown can visit them in reverse. On the
// first Boot error the sequence stops and the error is returned; providers
// booted before the failure are still recorded and will be shut down.
func (r *Registry) Boot(ctx context.Context, app Application) error {
	for _, p := range r.providers {
		if err := p.Boot(ctx, app); err != nil {
			return fmt.Errorf("provider: boot %T: %w", p, err)
		}

		r.booted = append(r.booted, p)
	}

	return nil
}

// Shutdown invokes Shutdown on booted providers in reverse order. Only
// providers that implement Shutdowner are visited. Shutdown errors are
// collected and returned as a joined error; every provider is given a
// chance to clean up even if earlier ones fail.
func (r *Registry) Shutdown(ctx context.Context, app Application) error {
	var errs []error

	for i := len(r.booted) - 1; i >= 0; i-- {
		p, ok := r.booted[i].(Shutdowner)
		if !ok {
			continue
		}

		if err := p.Shutdown(ctx, app); err != nil {
			errs = append(errs, fmt.Errorf("provider: shutdown %T: %w", p, err))
		}
	}

	r.booted = nil

	return errors.Join(errs...)
}
