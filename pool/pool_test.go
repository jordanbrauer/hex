package pool_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/pool"
)

func TestNew_basicSubmit(t *testing.T) {
	p := pool.New(2)

	var count int64

	for i := 0; i < 10; i++ {
		p.Submit(func() {
			atomic.AddInt64(&count, 1)
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := pool.Shutdown(ctx, p); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if got := atomic.LoadInt64(&count); got != 10 {
		t.Errorf("count = %d, want 10", got)
	}
}

func TestSubmitErr_returnsError(t *testing.T) {
	p := pool.New(2)
	defer pool.Shutdown(context.Background(), p)

	sentinel := errors.New("boom")

	task := p.SubmitErr(func() error {
		return sentinel
	})

	err := task.Wait()
	if !errors.Is(err, sentinel) {
		t.Errorf("Wait() = %v, want %v", err, sentinel)
	}
}

func TestSubmit_boundsConcurrency(t *testing.T) {
	const workers = 3

	p := pool.New(workers)
	defer pool.Shutdown(context.Background(), p)

	var running int64

	var peak int64

	release := make(chan struct{})

	for i := 0; i < 10; i++ {
		p.Submit(func() {
			n := atomic.AddInt64(&running, 1)
			for {
				old := atomic.LoadInt64(&peak)
				if n <= old || atomic.CompareAndSwapInt64(&peak, old, n) {
					break
				}
			}

			<-release
			atomic.AddInt64(&running, -1)
		})
	}

	// Give the pool time to reach steady state, then unblock.
	time.Sleep(50 * time.Millisecond)

	if got := atomic.LoadInt64(&peak); got > int64(workers) {
		close(release)
		t.Errorf("peak concurrency = %d, want <= %d", got, workers)

		return
	}

	close(release)
}

func TestTaskGroup_waitsForAll(t *testing.T) {
	p := pool.New(4)
	defer pool.Shutdown(context.Background(), p)

	group := p.NewGroup()

	var count int64

	for i := 0; i < 20; i++ {
		group.Submit(func() {
			atomic.AddInt64(&count, 1)
		})
	}

	if err := group.Wait(); err != nil {
		t.Errorf("Wait: %v", err)
	}

	if got := atomic.LoadInt64(&count); got != 20 {
		t.Errorf("count after Wait = %d, want 20", got)
	}
}

func TestNewResult_typedTask(t *testing.T) {
	rp := pool.NewResult[int](2)
	defer pool.Shutdown(context.Background(), rp)

	task := rp.SubmitErr(func() (int, error) {
		return 42, nil
	})

	got, err := task.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if got != 42 {
		t.Errorf("result = %d, want 42", got)
	}
}

func TestNewResult_errorPropagated(t *testing.T) {
	rp := pool.NewResult[string](2)
	defer pool.Shutdown(context.Background(), rp)

	sentinel := errors.New("nope")

	task := rp.SubmitErr(func() (string, error) {
		return "", sentinel
	})

	_, err := task.Wait()
	if !errors.Is(err, sentinel) {
		t.Errorf("Wait err = %v, want %v", err, sentinel)
	}
}

func TestShutdown_returnsAfterTasksFinish(t *testing.T) {
	p := pool.New(2)

	var done int64

	for i := 0; i < 5; i++ {
		p.Submit(func() {
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt64(&done, 1)
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := pool.Shutdown(ctx, p); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if got := atomic.LoadInt64(&done); got != 5 {
		t.Errorf("done = %d, want 5 (Shutdown should have waited)", got)
	}
}

func TestShutdown_respectsContextDeadline(t *testing.T) {
	p := pool.New(1)

	// One long-running task; Shutdown ctx expires before it finishes.
	p.Submit(func() { time.Sleep(500 * time.Millisecond) })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := pool.Shutdown(ctx, p)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Shutdown err = %v, want DeadlineExceeded", err)
	}
}

func TestSnapshot_capturesMetrics(t *testing.T) {
	p := pool.New(2)

	// Submit some tasks so counters are non-zero.
	for i := 0; i < 5; i++ {
		p.Submit(func() {})
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_ = pool.Shutdown(ctx, p)

	m := pool.Snapshot(p)

	if m.MaxConcurrency != 2 {
		t.Errorf("MaxConcurrency = %d, want 2", m.MaxConcurrency)
	}

	if m.SubmittedTasks != 5 {
		t.Errorf("SubmittedTasks = %d, want 5", m.SubmittedTasks)
	}

	if m.CompletedTasks != 5 {
		t.Errorf("CompletedTasks = %d, want 5", m.CompletedTasks)
	}

	if !m.Stopped {
		t.Errorf("Stopped = false after Shutdown")
	}

	if m.Timestamp.IsZero() {
		t.Errorf("Timestamp is zero")
	}
}

func TestPanic_isRecovered(t *testing.T) {
	// A panic in a submitted task must not tank the pool. Following tasks
	// still run.
	p := pool.New(2)

	var subsequent int64

	panicked := p.SubmitErr(func() error {
		panic("boom")
	})

	// Panics show up as errors from Wait().
	_ = panicked.Wait()

	// Submit a normal task after the panic.
	p.Submit(func() {
		atomic.AddInt64(&subsequent, 1)
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := pool.Shutdown(ctx, p); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if got := atomic.LoadInt64(&subsequent); got != 1 {
		t.Errorf("subsequent = %d, want 1 (pool must survive panic)", got)
	}
}

func TestWithQueueSize_boundsBacklog(t *testing.T) {
	// A tiny queue + tiny concurrency; TrySubmit past capacity returns
	// false when non-blocking.
	p := pool.New(1, pool.WithQueueSize(1), pool.WithNonBlocking(true))
	defer pool.Shutdown(context.Background(), p)

	block := make(chan struct{})

	// Occupy the worker.
	p.Submit(func() { <-block })

	// Fill queue (size 1).
	_, ok := p.TrySubmit(func() {})
	if !ok {
		t.Fatalf("second TrySubmit failed unexpectedly")
	}

	// Third should be rejected.
	_, ok = p.TrySubmit(func() {})
	if ok {
		t.Errorf("third TrySubmit accepted; queue not bounded")
	}

	close(block)
}
