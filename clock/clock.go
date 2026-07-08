// Package clock provides an injectable time source for testable code.
//
// Consumers that need to advance time in tests hold a Clock rather than
// calling time.Now directly. Production uses Real; tests use Frozen.
// Both satisfy the Clock interface.
//
// Package-level Now/Since/Sleep back onto a swappable default so
// consumers who do not want to thread a Clock through their code can
// still substitute one in tests via SetDefault.
package clock

import (
	"sync"
	"sync/atomic"
	"time"
)

// Clock is the injectable time source.
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	Sleep(d time.Duration)
}

// Real is a Clock backed by the standard time package.
type Real struct{}

// NewReal returns the singleton real clock.
func NewReal() Clock { return &Real{} }

// Now returns the current wall time.
func (Real) Now() time.Time { return time.Now() }

// Since returns time since t.
func (Real) Since(t time.Time) time.Duration { return time.Since(t) }

// Sleep pauses the caller for d.
func (Real) Sleep(d time.Duration) { time.Sleep(d) }

// Frozen is a Clock whose Now advances only when Advance is called.
// Sleep is a no-op unless Advance runs concurrently. Safe for concurrent
// use.
type Frozen struct {
	mu  sync.Mutex
	now time.Time
}

// NewFrozen returns a Frozen clock initialised to t.
func NewFrozen(t time.Time) *Frozen {
	return &Frozen{now: t}
}

// Now returns the current frozen time.
func (f *Frozen) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.now
}

// Since returns the difference between the frozen now and t.
func (f *Frozen) Since(t time.Time) time.Duration {
	return f.Now().Sub(t)
}

// Sleep is a no-op on the frozen clock. Tests that expect Sleep to
// influence time should call Advance instead.
func (f *Frozen) Sleep(time.Duration) {}

// Advance moves the frozen clock forward by d.
func (f *Frozen) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.now = f.now.Add(d)
}

// Set overrides the current time (useful for scenario tests).
func (f *Frozen) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.now = t
}

// -- package-level default -----------------------------------------------

//nolint:gochecknoglobals // package-level default clock is the whole point
var defaultClock atomic.Pointer[clockCell]

type clockCell struct{ c Clock }

func init() {
	SetDefault(NewReal())
}

// SetDefault installs c as the package-level clock. Test setup functions
// typically call SetDefault(NewFrozen(...)) then restore Real on cleanup.
func SetDefault(c Clock) {
	defaultClock.Store(&clockCell{c: c})
}

// Default returns the current package-level clock.
func Default() Clock {
	cell := defaultClock.Load()
	if cell == nil {
		return NewReal()
	}

	return cell.c
}

// Now returns the current time from the default clock.
func Now() time.Time { return Default().Now() }

// Since returns the elapsed time since t from the default clock.
func Since(t time.Time) time.Duration { return Default().Since(t) }

// Sleep pauses using the default clock.
func Sleep(d time.Duration) { Default().Sleep(d) }
