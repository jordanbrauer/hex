// Package retry provides generic exponential-backoff retry primitives.
//
// Use Do for the common "call this function up to N times with backoff"
// pattern. For richer control over the retry decision (some errors are
// terminal, others are transient), pass IsRetryable in Options.
//
// Example:
//
//	err := retry.Do(ctx, func(ctx context.Context) error {
//	    return svc.Sync(ctx)
//	}, retry.Options{
//	    MaxAttempts: 5,
//	    BaseDelay:   500 * time.Millisecond,
//	    MaxDelay:    30 * time.Second,
//	})
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"time"
)

// Options tunes a retry.
type Options struct {
	// MaxAttempts caps total attempts (initial + retries). Zero means 3.
	MaxAttempts int

	// BaseDelay is the initial delay before the second attempt. Zero
	// means 200ms.
	BaseDelay time.Duration

	// MaxDelay caps individual delays. Zero means 30s.
	MaxDelay time.Duration

	// Multiplier is the exponential base. Zero means 2 (each retry
	// doubles the previous delay).
	Multiplier float64

	// Jitter, in [0, 1], randomises the delay by ±(delay*jitter) to
	// reduce thundering herds. Zero disables jitter.
	Jitter float64

	// IsRetryable reports whether err should trigger a retry. Nil means
	// "retry on any non-nil error." Return false for permanent failures.
	IsRetryable func(err error) bool
}

// ErrPermanent is a sentinel callers can wrap when they know an error
// should abort retrying:
//
//	if err := doStuff(); err != nil && badRequest(err) {
//	    return fmt.Errorf("%w: %w", retry.ErrPermanent, err)
//	}
//
// Do treats any error wrapping ErrPermanent as terminal.
var ErrPermanent = errors.New("retry: permanent")

// Do executes fn up to Options.MaxAttempts times with exponential backoff.
// Returns the last error if all attempts fail, or nil on success. Honours
// context cancellation between attempts and during backoff sleeps.
func Do(ctx context.Context, fn func(context.Context) error, opts Options) error {
	opts = normalise(opts)

	var lastErr error

	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		if errors.Is(lastErr, ErrPermanent) {
			return lastErr
		}

		if opts.IsRetryable != nil && !opts.IsRetryable(lastErr) {
			return lastErr
		}

		if attempt == opts.MaxAttempts {
			break
		}

		delay := Backoff(attempt, opts)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

// Backoff returns the delay for the given attempt number (1-indexed).
// Exposed so consumers can compute the same schedule externally (e.g.
// for logging "will retry in Xs" hints).
func Backoff(attempt int, opts Options) time.Duration {
	opts = normalise(opts)

	if attempt < 1 {
		attempt = 1
	}

	shift := attempt - 1
	if shift > 30 {
		return jitter(opts.MaxDelay, opts.Jitter)
	}

	base := float64(opts.BaseDelay)
	mult := math.Pow(opts.Multiplier, float64(shift))
	d := time.Duration(base * mult)

	if d <= 0 || d > opts.MaxDelay {
		d = opts.MaxDelay
	}

	return jitter(d, opts.Jitter)
}

func jitter(d time.Duration, f float64) time.Duration {
	if f <= 0 {
		return d
	}

	if f > 1 {
		f = 1
	}

	// Symmetric jitter in [d*(1-f), d*(1+f)].
	//nolint:gosec // rand is fine for jitter; not a security decision.
	delta := (rand.Float64()*2 - 1) * f * float64(d)
	out := time.Duration(float64(d) + delta)

	if out < 0 {
		return 0
	}

	return out
}

func normalise(o Options) Options {
	if o.MaxAttempts == 0 {
		o.MaxAttempts = 3
	}

	if o.BaseDelay == 0 {
		o.BaseDelay = 200 * time.Millisecond
	}

	if o.MaxDelay == 0 {
		o.MaxDelay = 30 * time.Second
	}

	if o.Multiplier == 0 {
		o.Multiplier = 2
	}

	return o
}
