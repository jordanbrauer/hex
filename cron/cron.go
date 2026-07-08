// Package cron schedules recurring jobs using cron expressions.
//
// hex/cron wraps robfig/cron/v3 with a small facade that fits the hex
// provider lifecycle: consumers register jobs during Register/Boot, Start
// begins the ticker in the background, and Stop drains running jobs.
//
// A job is a function that takes a context and returns an error. Errors
// are logged; they never crash the scheduler. Panics inside a job are
// recovered so one bad job does not take down the rest.
//
// Example:
//
//	sched := cron.New()
//	if err := sched.Schedule("sync-releases", "*/5 * * * *", func(ctx context.Context) error {
//	    return svc.Sync(ctx)
//	}); err != nil {
//	    return err
//	}
//	sched.Start()
//	defer sched.Stop(ctx) // waits for in-flight jobs
package cron

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	robcron "github.com/robfig/cron/v3"

	hexlog "github.com/jordanbrauer/hex/log"
)

// Job is a function executed on a cron schedule. The context is derived from
// the scheduler's root context so shutdown cancellation reaches all jobs.
type Job func(context.Context) error

// Scheduler is the minimum surface consumers rely on. Concrete *Cron
// satisfies it; tests can substitute a fake without importing robfig.
type Scheduler interface {
	Schedule(name, spec string, job Job) error
	Remove(name string) bool
	Names() []string
}

// Cron is a hex-owned cron scheduler.
type Cron struct {
	mu     sync.Mutex
	cron   *robcron.Cron
	ctx    context.Context
	cancel context.CancelFunc
	byName map[string]robcron.EntryID
	opts   []robcron.Option
}

// Option configures a new Cron.
type Option func(*Cron)

// WithSeconds enables 6-field cron expressions (seconds precision). Without
// this, expressions are the standard 5-field minute-precision form.
func WithSeconds() Option {
	return func(c *Cron) {
		c.opts = append(c.opts, robcron.WithSeconds(),
			robcron.WithParser(robcron.NewParser(
				robcron.Second|robcron.Minute|robcron.Hour|
					robcron.Dom|robcron.Month|robcron.Dow|robcron.Descriptor,
			)))
	}
}

// New returns a Cron with the given options. Timezone defaults to time.Local;
// use robfig options via WithRobfigOption for advanced control.
func New(opts ...Option) *Cron {
	c := &Cron{
		byName: make(map[string]robcron.EntryID),
	}

	for _, opt := range opts {
		opt(c)
	}

	// Install a logger adapter so cron's internal messages flow through
	// hex/log at appropriate levels.
	c.opts = append(c.opts, robcron.WithLogger(newLogger()))
	c.cron = robcron.New(c.opts...)
	c.ctx, c.cancel = context.WithCancel(context.Background())

	return c
}

// Schedule registers job under name to run on spec. Returns an error if the
// spec is invalid or a job with the same name is already registered.
func (c *Cron) Schedule(name, spec string, job Job) error {
	if name == "" {
		return errors.New("cron: name is required")
	}

	if job == nil {
		return errors.New("cron: job is required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.byName[name]; exists {
		return fmt.Errorf("cron: job %q already registered", name)
	}

	id, err := c.cron.AddJob(spec, jobRunner{
		name: name,
		fn:   job,
		root: c.ctx,
	})
	if err != nil {
		return fmt.Errorf("cron: schedule %q: %w", name, err)
	}

	c.byName[name] = id

	return nil
}

// Remove unregisters the job with the given name. Returns true if a job was
// removed, false if no job with that name was registered.
func (c *Cron) Remove(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	id, ok := c.byName[name]
	if !ok {
		return false
	}

	c.cron.Remove(id)
	delete(c.byName, name)

	return true
}

// Names returns the sorted list of registered job names.
func (c *Cron) Names() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]string, 0, len(c.byName))
	for name := range c.byName {
		out = append(out, name)
	}

	// tiny n, hand-sort avoids importing sort
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}

	return out
}

// Start begins running scheduled jobs. Safe to call multiple times; only the
// first call starts the underlying ticker.
func (c *Cron) Start() {
	c.cron.Start()
}

// Stop signals the scheduler to stop accepting new job invocations and waits
// for in-flight jobs to finish or ctx to expire, whichever comes first.
// After Stop returns the scheduler cannot be restarted; construct a new
// Cron instead.
func (c *Cron) Stop(ctx context.Context) error {
	// Cancel the root context so jobs currently running see cancellation.
	c.cancel()

	done := c.cron.Stop().Done()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// jobRunner adapts hex's Job to robfig's cron.Job. It also recovers panics
// and logs errors so a single misbehaving job cannot topple the scheduler.
type jobRunner struct {
	name string
	fn   Job
	root context.Context
}

func (r jobRunner) Run() {
	defer func() {
		if rec := recover(); rec != nil {
			hexlog.Error("cron: job panic",
				"job", r.name,
				"panic", rec,
				"stack", string(debug.Stack()))
		}
	}()

	if err := r.fn(r.root); err != nil {
		hexlog.Error("cron: job failed", "job", r.name, "error", err)
	}
}
