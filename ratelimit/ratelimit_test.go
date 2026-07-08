package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/ratelimit"
)

func TestNew_burstAllowsInitial(t *testing.T) {
	// 1 rps, burst 3 → three immediate allows, next blocked.
	lim := ratelimit.New(1, 3)

	for i := 0; i < 3; i++ {
		if !lim.Allow() {
			t.Errorf("burst[%d] blocked", i)
		}
	}

	if lim.Allow() {
		t.Errorf("post-burst allowed without wait")
	}
}

func TestNew_zeroRPSMeansUnlimited(t *testing.T) {
	lim := ratelimit.New(0, 1)

	// Should allow indefinitely.
	for i := 0; i < 100; i++ {
		if !lim.Allow() {
			t.Errorf("rate=0 blocked at %d", i)

			return
		}
	}
}

func TestWait_blocksUntilToken(t *testing.T) {
	// 10 rps, burst 1 → one immediate, next blocks ~100ms.
	lim := ratelimit.New(10, 1)

	if !lim.Allow() {
		t.Fatalf("first Allow blocked")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()

	if err := lim.Wait(ctx); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("Wait returned too fast: %v", elapsed)
	}
}

func TestWait_contextTimeoutReturnsErr(t *testing.T) {
	lim := ratelimit.New(1, 1)

	_ = lim.Allow() // consume burst

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if err := lim.Wait(ctx); err == nil {
		t.Errorf("Wait returned nil with expired ctx")
	}
}

func TestNewFromInterval_correctPeriod(t *testing.T) {
	// One event every 50ms.
	lim := ratelimit.NewFromInterval(50*time.Millisecond, 1)

	if !lim.Allow() {
		t.Fatal("first Allow blocked")
	}

	// Immediately blocked.
	if lim.Allow() {
		t.Errorf("second Allow within interval")
	}
}

// -- KeyedLimiter --------------------------------------------------------

func TestKeyed_perKeyIndependence(t *testing.T) {
	kl := ratelimit.NewKeyed(1, 1, 100)

	if !kl.Allow("a") {
		t.Errorf("a[0] blocked")
	}

	if kl.Allow("a") {
		t.Errorf("a[1] allowed without wait")
	}

	// Different key, fresh bucket.
	if !kl.Allow("b") {
		t.Errorf("b[0] blocked; per-key isolation broken")
	}
}

func TestKeyed_evictsOldestWhenOverMax(t *testing.T) {
	kl := ratelimit.NewKeyed(1, 1, 2)

	_ = kl.Allow("a")

	time.Sleep(2 * time.Millisecond)

	_ = kl.Allow("b")

	time.Sleep(2 * time.Millisecond)

	_ = kl.Allow("c") // should evict a

	if kl.Len() != 2 {
		t.Errorf("Len = %d, want 2 (evicted)", kl.Len())
	}
}

func TestKeyed_touchOnAccessAvoidsEvictionOfActive(t *testing.T) {
	kl := ratelimit.NewKeyed(10, 10, 2)

	_ = kl.Allow("a")

	time.Sleep(2 * time.Millisecond)

	_ = kl.Allow("b")

	// Touch a again so it becomes newer than b.
	_ = kl.Allow("a")

	time.Sleep(2 * time.Millisecond)

	_ = kl.Allow("c") // should evict b, not a

	if !kl.Allow("a") {
		t.Errorf("a evicted despite recent touch")
	}
}

func TestKeyed_waitPerKey(t *testing.T) {
	kl := ratelimit.NewKeyed(10, 1, 100)

	_ = kl.Allow("k")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := kl.Wait(ctx, "k"); err != nil {
		t.Errorf("Wait: %v", err)
	}
}
