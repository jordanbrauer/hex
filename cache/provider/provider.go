// Package provider is the default hex/cache service provider.
//
// It picks a cache backend based on config and binds it into the
// container. v1 only understands the memory backend; add a Backend
// hook to plug in Redis/memcached/etc. from your app.
package provider

import (
	"github.com/jordanbrauer/hex/cache"
	"github.com/jordanbrauer/hex/cache/memory"
	"github.com/jordanbrauer/hex/config"
	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/provider"
)

// Provider binds a cache.Cache into the container.
type Provider struct {
	provider.Base

	// Binding is the container name for the cache. Defaults to "cache".
	Binding string

	// Namespace is the config namespace read for cache settings.
	// Defaults to "cache".
	Namespace string

	// Backend overrides the built-in backend selection. When set, the
	// factory function runs and its return value is bound directly —
	// the provider does not consult Namespace.driver in that case.
	Backend func() cache.Cache

	cache cache.Cache
}

// Register selects the backend and binds it.
func (p *Provider) Register(app provider.Application) error {
	binding := p.Binding
	if binding == "" {
		binding = "cache"
	}

	if p.Backend != nil {
		p.cache = p.Backend()
	} else {
		p.cache = p.buildBackend()
	}

	app.Singleton(binding, func(*container.Container) (any, error) {
		return p.cache, nil
	})

	return nil
}

// buildBackend consults config to pick a backend. v1 recognises
// "memory" (default). Unknown drivers fall back to memory with a
// warning logged by the caller (we cannot log here without importing
// hex/log; consumers who want stricter behaviour set Backend).
func (p *Provider) buildBackend() cache.Cache {
	ns := p.Namespace
	if ns == "" {
		ns = "cache"
	}

	switch config.String(ns + ".driver") {
	case "", "memory":
		return memory.New()
	default:
		// Unknown → memory. Consumer factories can override via Backend.
		return memory.New()
	}
}
