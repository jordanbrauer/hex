// Package ratelimit is a thin wrapper around golang.org/x/time/rate.
//
// A Limiter caps a rate of events with token-bucket semantics: Allow
// checks if a token is available now, Wait blocks until one is, and
// Reserve returns a delay hint without blocking.
//
// A KeyedLimiter maintains a Limiter per key (e.g. per user, per IP)
// with an LRU-like eviction to bound memory.
//
// Example:
//
//	// 100 requests/sec, burst of 200
//	lim := ratelimit.New(100, 200)
//
//	if err := lim.Wait(ctx); err != nil { return err }
//	callAPI()
package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter is the type alias for x/time/rate.Limiter. Consumers get the
// full upstream API through the alias.
type Limiter = rate.Limiter

// New returns a Limiter with rate rps events per second and burst
// capacity. Zero rate means unlimited.
func New(rps float64, burst int) *Limiter {
	if rps <= 0 {
		return rate.NewLimiter(rate.Inf, burst)
	}

	return rate.NewLimiter(rate.Limit(rps), burst)
}

// NewFromInterval builds a Limiter that admits one event every d.
func NewFromInterval(d time.Duration, burst int) *Limiter {
	if d <= 0 {
		return rate.NewLimiter(rate.Inf, burst)
	}

	return rate.NewLimiter(rate.Every(d), burst)
}

// -- KeyedLimiter --------------------------------------------------------

// KeyedLimiter holds one Limiter per string key. Bounded by MaxKeys to
// prevent unbounded memory growth on high-cardinality inputs (e.g. IPs).
// Eviction is simple last-touched: when the limit is exceeded, the
// least-recently-used key drops. Not strictly LRU — the goal is a memory
// cap, not perfect eviction accuracy.
type KeyedLimiter struct {
	mu       sync.Mutex
	rps      float64
	burst    int
	maxKeys  int
	limiters map[string]*keyedEntry
	order    []string // append on touch, dedupe on eviction
}

type keyedEntry struct {
	limiter  *Limiter
	touched  time.Time
	position int // index into KeyedLimiter.order for fast update
}

// NewKeyed returns a KeyedLimiter. rps and burst apply to every per-key
// limiter. maxKeys caps memory; zero means unbounded (not recommended).
func NewKeyed(rps float64, burst, maxKeys int) *KeyedLimiter {
	return &KeyedLimiter{
		rps:      rps,
		burst:    burst,
		maxKeys:  maxKeys,
		limiters: make(map[string]*keyedEntry),
	}
}

// Allow reports whether an event with key may proceed now, consuming
// one token if so.
func (k *KeyedLimiter) Allow(key string) bool {
	l := k.limiterFor(key)

	return l.Allow()
}

// Wait blocks until an event with key can proceed, or ctx expires.
func (k *KeyedLimiter) Wait(ctx context.Context, key string) error {
	l := k.limiterFor(key)

	return l.Wait(ctx)
}

// Reserve returns a *rate.Reservation for key. Use r.Delay() to see how
// long the caller should wait, or r.Cancel() to release the token.
func (k *KeyedLimiter) Reserve(key string) *rate.Reservation {
	l := k.limiterFor(key)

	return l.Reserve()
}

// Len returns the number of tracked keys. Useful for tests and metrics.
func (k *KeyedLimiter) Len() int {
	k.mu.Lock()
	defer k.mu.Unlock()

	return len(k.limiters)
}

// limiterFor returns (creating if absent) the per-key limiter, evicting
// the oldest key if MaxKeys is exceeded.
func (k *KeyedLimiter) limiterFor(key string) *Limiter {
	k.mu.Lock()
	defer k.mu.Unlock()

	now := time.Now()

	if e, ok := k.limiters[key]; ok {
		e.touched = now

		return e.limiter
	}

	// Evict if over capacity.
	if k.maxKeys > 0 && len(k.limiters) >= k.maxKeys {
		k.evictOldest()
	}

	lim := New(k.rps, k.burst)
	k.limiters[key] = &keyedEntry{limiter: lim, touched: now}

	return lim
}

// evictOldest walks the map and drops the least-recently-touched key.
// O(n) but n is bounded by MaxKeys and this only fires on the eviction
// path, not the hot allow path.
func (k *KeyedLimiter) evictOldest() {
	var (
		oldestKey  string
		oldestTime time.Time
	)

	for key, e := range k.limiters {
		if oldestKey == "" || e.touched.Before(oldestTime) {
			oldestKey = key
			oldestTime = e.touched
		}
	}

	if oldestKey != "" {
		delete(k.limiters, oldestKey)
	}
}
