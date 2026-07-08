package cron_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/cron"
)

func TestSchedule_requiresName(t *testing.T) {
	c := cron.New()
	if err := c.Schedule("", "* * * * *", func(context.Context) error { return nil }); err == nil {
		t.Errorf("Schedule with empty name returned nil error")
	}
}

func TestSchedule_requiresJob(t *testing.T) {
	c := cron.New()
	if err := c.Schedule("x", "* * * * *", nil); err == nil {
		t.Errorf("Schedule with nil job returned nil error")
	}
}

func TestSchedule_rejectsDuplicateName(t *testing.T) {
	c := cron.New()

	if err := c.Schedule("x", "* * * * *", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("first Schedule error = %v", err)
	}

	if err := c.Schedule("x", "*/2 * * * *", func(context.Context) error { return nil }); err == nil {
		t.Errorf("duplicate Schedule returned nil error")
	}
}

func TestSchedule_rejectsInvalidSpec(t *testing.T) {
	c := cron.New()

	if err := c.Schedule("x", "not a cron spec", func(context.Context) error { return nil }); err == nil {
		t.Errorf("Schedule with bad spec returned nil error")
	}
}

func TestNames_returnsSorted(t *testing.T) {
	c := cron.New()

	_ = c.Schedule("beta", "* * * * *", func(context.Context) error { return nil })
	_ = c.Schedule("alpha", "* * * * *", func(context.Context) error { return nil })
	_ = c.Schedule("gamma", "* * * * *", func(context.Context) error { return nil })

	got := c.Names()
	want := []string{"alpha", "beta", "gamma"}

	if len(got) != 3 {
		t.Fatalf("Names = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Names[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRemove(t *testing.T) {
	c := cron.New()

	_ = c.Schedule("x", "* * * * *", func(context.Context) error { return nil })

	if !c.Remove("x") {
		t.Errorf("Remove(existing) = false, want true")
	}

	if c.Remove("x") {
		t.Errorf("Remove(missing) = true, want false")
	}

	if len(c.Names()) != 0 {
		t.Errorf("Names after Remove = %v, want empty", c.Names())
	}
}

func TestStartStop_runsJob(t *testing.T) {
	// Use WithSeconds so we can schedule "every second" and observe execution
	// within a reasonable test wall clock.
	c := cron.New(cron.WithSeconds())

	var runs int64
	done := make(chan struct{}, 1)

	if err := c.Schedule("tick", "* * * * * *", func(context.Context) error {
		atomic.AddInt64(&runs, 1)

		select {
		case done <- struct{}{}:
		default:
		}

		return nil
	}); err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	c.Start()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not run within 3s")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.Stop(ctx); err != nil {
		t.Errorf("Stop error = %v", err)
	}

	if atomic.LoadInt64(&runs) < 1 {
		t.Errorf("runs = %d, want >= 1", runs)
	}
}

func TestJobPanic_isRecovered(t *testing.T) {
	c := cron.New(cron.WithSeconds())

	var (
		panicked int64
		normal   int64
	)

	done := make(chan struct{}, 2)

	_ = c.Schedule("panicky", "* * * * * *", func(context.Context) error {
		atomic.AddInt64(&panicked, 1)

		select {
		case done <- struct{}{}:
		default:
		}

		panic("boom")
	})

	_ = c.Schedule("normal", "* * * * * *", func(context.Context) error {
		atomic.AddInt64(&normal, 1)

		select {
		case done <- struct{}{}:
		default:
		}

		return nil
	})

	c.Start()

	// Wait for both jobs to fire at least once.
	deadline := time.After(4 * time.Second)

	for atomic.LoadInt64(&panicked) == 0 || atomic.LoadInt64(&normal) == 0 {
		select {
		case <-done:
		case <-deadline:
			t.Fatalf("timed out; panicked=%d normal=%d", panicked, normal)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.Stop(ctx); err != nil {
		t.Errorf("Stop error = %v", err)
	}
}

func TestJobError_doesNotCrash(t *testing.T) {
	c := cron.New(cron.WithSeconds())

	var runs int64
	done := make(chan struct{}, 1)
	sentinel := errors.New("expected")

	_ = c.Schedule("errs", "* * * * * *", func(context.Context) error {
		atomic.AddInt64(&runs, 1)

		select {
		case done <- struct{}{}:
		default:
		}

		return sentinel
	})

	c.Start()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.Stop(ctx); err != nil {
		t.Errorf("Stop error = %v", err)
	}

	if atomic.LoadInt64(&runs) < 1 {
		t.Errorf("runs = %d, want >= 1", runs)
	}
}

func TestStop_cancelsJobContext(t *testing.T) {
	c := cron.New(cron.WithSeconds())

	started := make(chan struct{}, 1)
	saw := make(chan error, 1)

	_ = c.Schedule("slow", "* * * * * *", func(ctx context.Context) error {
		select {
		case started <- struct{}{}:
		default:
		}

		select {
		case <-ctx.Done():
			saw <- ctx.Err()
		case <-time.After(5 * time.Second):
			saw <- errors.New("no cancel")
		}

		return nil
	})

	c.Start()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not start")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = c.Stop(stopCtx)

	select {
	case err := <-saw:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("job saw %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("job never observed cancel")
	}
}

func TestScheduler_interface(t *testing.T) {
	// Compile-time proof *Cron satisfies Scheduler.
	var _ cron.Scheduler = cron.New()
}
