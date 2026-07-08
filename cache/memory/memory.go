// Package memory is an in-process cache backend. It stores values in a
// sync-protected map, evicts on TTL check, and is safe for concurrent use.
//
// Memory is the default backend for tests and single-process applications
// that do not need cross-process cache coherence. For multi-instance
// deployments use hex/cache/redis or hex/cache/memcached (opt-in).
package memory

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/jordanbrauer/hex/cache"
)

// Cache is an in-memory cache implementation.
type Cache struct {
	mu    sync.RWMutex
	items map[string]entry
	now   func() time.Time // injectable clock for tests
}

type entry struct {
	value     []byte
	expiresAt time.Time // zero = no expiration
}

// New returns an empty in-memory cache.
func New() *Cache {
	return &Cache{
		items: make(map[string]entry),
		now:   time.Now,
	}
}

// WithClock returns a Cache using now for time. Intended for tests that need
// to advance time deterministically instead of sleeping.
func WithClock(now func() time.Time) *Cache {
	return &Cache{
		items: make(map[string]entry),
		now:   now,
	}
}

// Get returns the value for key, or (nil, false, nil) on miss/expiry.
func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false, nil
	}

	if !e.expiresAt.IsZero() && !c.now().Before(e.expiresAt) {
		// Expired. Clean up lazily.
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()

		return nil, false, nil
	}

	// Defensive copy so callers cannot mutate our stored slice.
	out := make([]byte, len(e.value))
	copy(out, e.value)

	return out, true, nil
}

// Set stores value under key. See package cache docs for TTL semantics.
func (c *Cache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl < 0 {
		return cache.ErrNegativeTTL
	}

	stored := make([]byte, len(value))
	copy(stored, value)

	var expires time.Time
	if ttl > 0 {
		expires = c.now().Add(ttl)
	}

	c.mu.Lock()
	c.items[key] = entry{value: stored, expiresAt: expires}
	c.mu.Unlock()

	return nil
}

// Delete removes key.
func (c *Cache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()

	return nil
}

// Has reports whether key is present and not expired.
func (c *Cache) Has(ctx context.Context, key string) (bool, error) {
	_, hit, err := c.Get(ctx, key)

	return hit, err
}

// Clear removes all entries.
func (c *Cache) Clear(context.Context) error {
	c.mu.Lock()
	c.items = make(map[string]entry)
	c.mu.Unlock()

	return nil
}

// Increment atomically adds delta to the value at key. Absent keys start
// from zero and take no expiration. Non-numeric existing values return an
// error.
func (c *Cache) Increment(_ context.Context, key string, delta int64) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var current int64

	if e, ok := c.items[key]; ok {
		if !e.expiresAt.IsZero() && !c.now().Before(e.expiresAt) {
			delete(c.items, key)
		} else {
			n, err := strconv.ParseInt(string(e.value), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("cache: value at %q is not numeric: %w", key, err)
			}

			current = n
		}
	}

	current += delta
	c.items[key] = entry{value: []byte(strconv.FormatInt(current, 10))}

	return current, nil
}

// Len returns the number of stored entries (including expired-but-not-yet-
// swept ones). Useful for tests and diagnostics.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}

// compile-time proof of interface conformance
var _ cache.Cache = (*Cache)(nil)
