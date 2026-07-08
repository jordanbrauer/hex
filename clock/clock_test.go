package clock_test

import (
	"testing"
	"time"

	"github.com/jordanbrauer/hex/clock"
)

func TestReal_returnsCurrentTime(t *testing.T) {
	c := clock.NewReal()

	got := c.Now()
	if time.Since(got) > time.Second {
		t.Errorf("Now = %v, too far in past", got)
	}
}

func TestFrozen_nowStable(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := clock.NewFrozen(fixed)

	for i := 0; i < 3; i++ {
		if got := f.Now(); !got.Equal(fixed) {
			t.Errorf("iteration %d: Now = %v, want %v", i, got, fixed)
		}
	}
}

func TestFrozen_advance(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := clock.NewFrozen(start)

	f.Advance(time.Hour)

	if got := f.Now(); !got.Equal(start.Add(time.Hour)) {
		t.Errorf("after Advance(1h) = %v, want %v", got, start.Add(time.Hour))
	}
}

func TestFrozen_set(t *testing.T) {
	f := clock.NewFrozen(time.Unix(0, 0))

	target := time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC)
	f.Set(target)

	if got := f.Now(); !got.Equal(target) {
		t.Errorf("after Set: %v, want %v", got, target)
	}
}

func TestFrozen_since(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := clock.NewFrozen(start.Add(time.Hour))

	if got := f.Since(start); got != time.Hour {
		t.Errorf("Since = %v, want 1h", got)
	}
}

func TestFrozen_sleepIsNoop(t *testing.T) {
	f := clock.NewFrozen(time.Unix(0, 0))
	before := time.Now()

	f.Sleep(time.Second)

	if elapsed := time.Since(before); elapsed > 10*time.Millisecond {
		t.Errorf("Frozen.Sleep took %v, expected no-op", elapsed)
	}
}

func TestPackage_defaultIsReal(t *testing.T) {
	if got := clock.Now(); time.Since(got) > time.Second {
		t.Errorf("package Now default = %v (not real?)", got)
	}
}

func TestPackage_setDefaultSwapsClock(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f := clock.NewFrozen(fixed)

	prev := clock.Default()

	clock.SetDefault(f)

	t.Cleanup(func() { clock.SetDefault(prev) })

	if got := clock.Now(); !got.Equal(fixed) {
		t.Errorf("after SetDefault(frozen): Now = %v, want %v", got, fixed)
	}
}
