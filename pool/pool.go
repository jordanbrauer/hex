// Package pool is a thin wrapper around github.com/alitto/pond/v2 that
// gives hex applications a worker pool primitive for bounded in-process
// concurrency.
//
// Use hex/pool when you need to fan out N tasks across a bounded number
// of goroutines. It is orthogonal to hex/queue (delivery + durability)
// and hex/cron (timing): a queue consumer, cron job, or HTTP handler can
// all use a pool to run their work concurrently.
//
// See ADR-0010 for the wrap-vs-roll-own decision.
//
// Example:
//
//	p := pool.New(10)                              // 10-worker pool
//	defer p.Shutdown(ctx)                          // waits for in-flight
//
//	for _, item := range items {
//	    item := item
//	    p.Submit(func() { process(item) })
//	}
//
//	// Or with results:
//	rp := pool.NewResult[User](10)
//	task := rp.SubmitErr(func() (User, error) {
//	    return svc.Get(ctx, id)
//	})
//	user, err := task.Wait()
package pool

import (
	"context"
	"time"

	"github.com/alitto/pond/v2"
)

// Pool is the type alias for pond's non-generic pool. Consumers get the
// full pond API through the alias.
type Pool = pond.Pool

// ResultPool is the type alias for pond's typed result pool.
type ResultPool[R any] = pond.ResultPool[R]

// Task represents a submitted unit of work with a Wait() method.
type Task = pond.Task

// ResultTask represents a submitted task that returns a typed result.
type ResultTask[R any] = pond.Result[R]

// TaskGroup batches multiple tasks and lets the caller Wait for all of
// them at once.
type TaskGroup = pond.TaskGroup

// Option configures a new pool. Aliased from pond so callers can pass
// upstream options directly if they need something the hex helpers do
// not surface.
type Option = pond.Option

// -- Convenience option helpers ------------------------------------------
//
// hex/pool re-exports the pond options with hex-consistent naming so
// callers do not need to import pond in typical usage.

// WithQueueSize caps the number of pending tasks the pool will accept
// before blocking (or dropping, with WithNonBlocking).
func WithQueueSize(n int) Option { return pond.WithQueueSize(n) }

// WithNonBlocking makes Submit drop tasks that cannot be queued instead
// of blocking. Combine with WithQueueSize.
func WithNonBlocking(v bool) Option { return pond.WithNonBlocking(v) }

// WithContext sets a base context on the pool. When ctx is cancelled the
// pool stops accepting new tasks and returns ErrPoolStopped.
func WithContext(ctx context.Context) Option { return pond.WithContext(ctx) }

// -- Constructors --------------------------------------------------------

// New returns a Pool with the given maximum concurrency. Zero means
// unlimited (goroutine per task; usually not what you want).
func New(maxConcurrency int, opts ...Option) Pool {
	return pond.NewPool(maxConcurrency, opts...)
}

// NewResult returns a typed ResultPool that accepts tasks returning R.
// Use this when you need to collect return values or errors from tasks.
func NewResult[R any](maxConcurrency int, opts ...Option) ResultPool[R] {
	return pond.NewResultPool[R](maxConcurrency, opts...)
}

// -- Lifecycle -----------------------------------------------------------

// Shutdown stops the pool gracefully. It stops accepting new tasks and
// waits for in-flight tasks to complete, up to ctx's deadline.
//
// Returns nil on clean drain, ctx.Err() if the deadline is hit before
// tasks finish.
func Shutdown(ctx context.Context, p BasePool) error {
	done := make(chan struct{})

	go func() {
		p.StopAndWait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BasePool is the minimum interface every pond pool satisfies. Used by
// Shutdown and Snapshot so the same helpers work for Pool and
// ResultPool.
type BasePool interface {
	StopAndWait()
	Stop() pond.Task

	RunningWorkers() int64
	SubmittedTasks() uint64
	WaitingTasks() uint64
	SuccessfulTasks() uint64
	FailedTasks() uint64
	CompletedTasks() uint64
	DroppedTasks() uint64
	CanceledTasks() uint64

	MaxConcurrency() int
	QueueSize() int
	Stopped() bool
}

// Metrics is a point-in-time snapshot of a pool's counters. Read via
// Snapshot for consumers that want to expose pool health via
// /healthz-adjacent endpoints or metrics scrapers.
type Metrics struct {
	Timestamp       time.Time
	MaxConcurrency  int
	QueueSize       int
	RunningWorkers  int64
	SubmittedTasks  uint64
	WaitingTasks    uint64
	SuccessfulTasks uint64
	FailedTasks     uint64
	CompletedTasks  uint64
	DroppedTasks    uint64
	CanceledTasks   uint64
	Stopped         bool
}

// Snapshot reads all counters from p at once.
func Snapshot(p BasePool) Metrics {
	return Metrics{
		Timestamp:       time.Now(),
		MaxConcurrency:  p.MaxConcurrency(),
		QueueSize:       p.QueueSize(),
		RunningWorkers:  p.RunningWorkers(),
		SubmittedTasks:  p.SubmittedTasks(),
		WaitingTasks:    p.WaitingTasks(),
		SuccessfulTasks: p.SuccessfulTasks(),
		FailedTasks:     p.FailedTasks(),
		CompletedTasks:  p.CompletedTasks(),
		DroppedTasks:    p.DroppedTasks(),
		CanceledTasks:   p.CanceledTasks(),
		Stopped:         p.Stopped(),
	}
}
