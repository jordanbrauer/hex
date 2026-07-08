package retry_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/retry"
)

func TestDo_succeedsOnFirstTry(t *testing.T) {
	var attempts int64

	err := retry.Do(context.Background(), func(context.Context) error {
		atomic.AddInt64(&attempts, 1)

		return nil
	}, retry.Options{})

	if err != nil {
		t.Errorf("Do = %v, want nil", err)
	}

	if got := atomic.LoadInt64(&attempts); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestDo_retriesUntilSuccess(t *testing.T) {
	var attempts int64

	err := retry.Do(context.Background(), func(context.Context) error {
		n := atomic.AddInt64(&attempts, 1)
		if n < 3 {
			return errors.New("transient")
		}

		return nil
	}, retry.Options{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	})

	if err != nil {
		t.Errorf("Do = %v, want nil", err)
	}

	if got := atomic.LoadInt64(&attempts); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestDo_returnsLastError(t *testing.T) {
	var attempts int64

	sentinel := errors.New("persistent")

	err := retry.Do(context.Background(), func(context.Context) error {
		atomic.AddInt64(&attempts, 1)

		return sentinel
	}, retry.Options{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
	})

	if !errors.Is(err, sentinel) {
		t.Errorf("Do = %v, want %v", err, sentinel)
	}

	if got := atomic.LoadInt64(&attempts); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestDo_permanentSkipsRetry(t *testing.T) {
	var attempts int64

	err := retry.Do(context.Background(), func(context.Context) error {
		atomic.AddInt64(&attempts, 1)

		return fmt.Errorf("wrap: %w", retry.ErrPermanent)
	}, retry.Options{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
	})

	if !errors.Is(err, retry.ErrPermanent) {
		t.Errorf("Do = %v, want ErrPermanent", err)
	}

	if got := atomic.LoadInt64(&attempts); got != 1 {
		t.Errorf("attempts = %d, want 1 (permanent short-circuit)", got)
	}
}

func TestDo_isRetryablePredicate(t *testing.T) {
	var attempts int64

	notRetryable := errors.New("do not retry")

	err := retry.Do(context.Background(), func(context.Context) error {
		atomic.AddInt64(&attempts, 1)

		return notRetryable
	}, retry.Options{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
		IsRetryable: func(err error) bool { return !errors.Is(err, notRetryable) },
	})

	if err != notRetryable {
		t.Errorf("Do = %v, want %v", err, notRetryable)
	}

	if got := atomic.LoadInt64(&attempts); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestDo_contextCancelStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var attempts int64

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	err := retry.Do(ctx, func(context.Context) error {
		atomic.AddInt64(&attempts, 1)

		return errors.New("keep trying")
	}, retry.Options{
		MaxAttempts: 100,
		BaseDelay:   20 * time.Millisecond,
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Do err = %v, want Canceled", err)
	}
}

func TestBackoff_exponentialGrowthWithCap(t *testing.T) {
	opts := retry.Options{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   time.Second,
		Multiplier: 2,
	}

	tests := []struct {
		attempt int
		min     time.Duration
		max     time.Duration
	}{
		{1, 100 * time.Millisecond, 100 * time.Millisecond},
		{2, 200 * time.Millisecond, 200 * time.Millisecond},
		{3, 400 * time.Millisecond, 400 * time.Millisecond},
		{4, 800 * time.Millisecond, 800 * time.Millisecond},
		{5, time.Second, time.Second}, // capped at MaxDelay
		{6, time.Second, time.Second},
	}

	for _, tt := range tests {
		got := retry.Backoff(tt.attempt, opts)
		if got < tt.min || got > tt.max {
			t.Errorf("Backoff(%d) = %v, want [%v,%v]", tt.attempt, got, tt.min, tt.max)
		}
	}
}

func TestBackoff_jitter(t *testing.T) {
	opts := retry.Options{
		BaseDelay:  time.Second,
		MaxDelay:   time.Second,
		Multiplier: 1,
		Jitter:     0.5,
	}

	// With 50% jitter, results must fall in [0.5s, 1.5s].
	for i := 0; i < 100; i++ {
		d := retry.Backoff(1, opts)
		if d < 500*time.Millisecond || d > 1500*time.Millisecond {
			t.Errorf("Backoff jittered = %v, out of range", d)
		}
	}
}
