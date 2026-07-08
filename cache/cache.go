// Package cache defines a driver-agnostic key-value cache with TTL semantics.
//
// The Cache interface is byte-oriented so backends (memory, Redis, memcached)
// can all implement it without conversion assumptions. Generic helpers
// Get, Set, and Remember layer JSON serialization on top for typed values.
//
// A hex app typically resolves caches by name from the container:
//
//	c := container.Must[cache.Cache](app.Container(), "cache.default")
//	if err := cache.Set(ctx, c, "user:1", user, 5*time.Minute); err != nil {
//	    return err
//	}
//	user, hit, err := cache.Get[User](ctx, c, "user:1")
//
// TTL semantics:
//
//   - ttl > 0: the entry expires after that duration.
//   - ttl == 0: the entry does not expire (persistent for the cache's lifetime).
//   - ttl < 0: reserved; drivers must return an error.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrMiss is returned by helpers when they need to distinguish "not present"
// from "backend error." The raw Cache interface reports misses via the hit
// bool instead.
var ErrMiss = errors.New("cache: miss")

// ErrNegativeTTL is returned by Set when ttl < 0.
var ErrNegativeTTL = errors.New("cache: negative TTL")

// Cache is the driver-facing cache interface. Implementations must be safe
// for concurrent use.
type Cache interface {
	// Get returns the value for key. The hit bool is false (with nil value
	// and nil error) if the key is absent or expired. A non-nil error
	// indicates a backend failure, not a miss.
	Get(ctx context.Context, key string) (value []byte, hit bool, err error)

	// Set writes value under key with the given TTL. See package docs for
	// TTL semantics.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes key. Removing a key that does not exist is not an error.
	Delete(ctx context.Context, key string) error

	// Has reports whether key is present and not expired.
	Has(ctx context.Context, key string) (bool, error)

	// Clear removes all entries. Optional for drivers that treat this as
	// destructive (e.g. shared Redis); such drivers may return an error.
	Clear(ctx context.Context) error

	// Increment atomically adds delta to the numeric value at key and
	// returns the new value. If key does not exist, it is created with
	// value delta (and no expiration). Non-numeric existing values return
	// an error.
	Increment(ctx context.Context, key string, delta int64) (int64, error)
}

// Get resolves key from c and JSON-decodes the stored bytes into T. The hit
// bool is false when the key is absent; err is set only on backend or
// decode failure.
func Get[T any](ctx context.Context, c Cache, key string) (T, bool, error) {
	var zero T

	raw, hit, err := c.Get(ctx, key)
	if err != nil {
		return zero, false, err
	}

	if !hit {
		return zero, false, nil
	}

	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, true, fmt.Errorf("cache: decode %q as %T: %w", key, zero, err)
	}

	return out, true, nil
}

// Set JSON-encodes value and writes it to c under key with the given TTL.
func Set[T any](ctx context.Context, c Cache, key string, value T, ttl time.Duration) error {
	if ttl < 0 {
		return ErrNegativeTTL
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache: encode %q: %w", key, err)
	}

	return c.Set(ctx, key, raw, ttl)
}

// Remember returns the value at key if present. Otherwise it calls fn,
// stores the result under key with ttl, and returns it. Concurrent callers
// may cause fn to be invoked more than once — this is a cache, not a lock.
// Consumers that need single-flight behaviour should compose with
// golang.org/x/sync/singleflight.
func Remember[T any](ctx context.Context, c Cache, key string, ttl time.Duration, fn func(context.Context) (T, error)) (T, error) {
	var zero T

	if val, hit, err := Get[T](ctx, c, key); err != nil {
		return zero, err
	} else if hit {
		return val, nil
	}

	val, err := fn(ctx)
	if err != nil {
		return zero, err
	}

	if err := Set(ctx, c, key, val, ttl); err != nil {
		// Value is fine, cache is not. Return the value and swallow the
		// storage error at the caller's request... except we don't know
		// their preference. Return the error so the caller can log-and-
		// return-val if they want to.
		return val, err
	}

	return val, nil
}
