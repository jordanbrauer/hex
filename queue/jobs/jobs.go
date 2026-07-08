// Package jobs is a typed job layer on top of hex/queue. It gives
// consumers Laravel/Sidekiq-style named jobs with retry, exponential
// backoff, dead-letter routing, and delayed dispatch — all built on any
// queue.Queue implementation.
//
// A job is a struct that satisfies the Job interface: it has a Name and
// a Run method. Consumers register handlers on a Runner; Dispatch
// serialises a job into a queue.Message and publishes it. On the
// consumer side the Runner dispatches to the registered handler for the
// job's Name, applies retry policy, and moves permanently failing jobs
// to a dead-letter topic.
//
// Envelope wire format:
//
//	{"name": "send-email", "payload": <json>, "attempt": 3, "dispatched_at": "..."}
//
// The envelope is JSON so the wire is inspectable and portable across
// backends. Payload is the arbitrary Go value the consumer passes.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	hexlog "github.com/jordanbrauer/hex/log"
	"github.com/jordanbrauer/hex/queue"
)

// Envelope is the wire format for a queued job. It is exported so tools
// (dashboards, dead-letter inspectors) can decode messages without a
// dependency on hex/queue/jobs.
type Envelope struct {
	Name         string          `json:"name"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Attempt      int             `json:"attempt"`
	MaxAttempts  int             `json:"max_attempts,omitempty"`
	DispatchedAt time.Time       `json:"dispatched_at"`
	ID           string          `json:"id,omitempty"`
}

// Handler is invoked when a matching job is received. The raw payload is
// passed as JSON bytes; handlers typically unmarshal into their own
// payload struct.
type Handler func(ctx context.Context, payload json.RawMessage) error

// Options tune a Runner and its dispatched jobs.
type Options struct {
	// Topic is the queue.Queue topic used for all jobs handled by this
	// runner. Defaults to "jobs".
	Topic string

	// DeadLetterTopic is where jobs go after MaxAttempts. Empty disables
	// dead-lettering; failing jobs are dropped instead.
	DeadLetterTopic string

	// MaxAttempts caps the total number of attempts per job (initial
	// try + retries). Zero means 3. Set to a negative number to retry
	// forever (not recommended).
	MaxAttempts int

	// BaseBackoff is the initial retry delay. Defaults to 1 second.
	// Exponential: attempt n waits BaseBackoff * 2^(n-1), capped at
	// MaxBackoff.
	BaseBackoff time.Duration

	// MaxBackoff caps the retry delay. Defaults to 5 minutes.
	MaxBackoff time.Duration
}

// Runner registers job handlers and consumes them from the underlying
// queue.
type Runner struct {
	q        queue.Queue
	opts     Options
	handlers sync.Map // name -> Handler

	subMu sync.Mutex
	sub   queue.Subscription
}

// NewRunner constructs a Runner. Call Register for every job type before
// Start.
func NewRunner(q queue.Queue, opts Options) *Runner {
	if opts.Topic == "" {
		opts.Topic = "jobs"
	}

	if opts.MaxAttempts == 0 {
		opts.MaxAttempts = 3
	}

	if opts.BaseBackoff == 0 {
		opts.BaseBackoff = time.Second
	}

	if opts.MaxBackoff == 0 {
		opts.MaxBackoff = 5 * time.Minute
	}

	return &Runner{q: q, opts: opts}
}

// Register installs a handler for the given job name. Registering the
// same name twice overwrites the previous handler.
func (r *Runner) Register(name string, handler Handler) {
	r.handlers.Store(name, handler)
}

// Dispatch serialises payload and publishes a job with the given name to
// the runner's topic. opts customise this single dispatch.
type DispatchOptions struct {
	// Delay defers execution for at least this duration.
	Delay time.Duration

	// MaxAttempts overrides the runner default for this specific job.
	MaxAttempts int
}

// Dispatch publishes a job. Payload is any JSON-marshallable value. The
// returned string is the underlying message ID.
func (r *Runner) Dispatch(ctx context.Context, name string, payload any, opts ...DispatchOptions) (string, error) {
	if name == "" {
		return "", errors.New("jobs: name is required")
	}

	var raw json.RawMessage

	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("jobs: marshal payload for %q: %w", name, err)
		}

		raw = b
	}

	env := Envelope{
		Name:         name,
		Payload:      raw,
		Attempt:      0,
		DispatchedAt: time.Now(),
	}

	var d DispatchOptions
	if len(opts) > 0 {
		d = opts[0]
	}

	if d.MaxAttempts > 0 {
		env.MaxAttempts = d.MaxAttempts
	}

	body, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("jobs: marshal envelope for %q: %w", name, err)
	}

	pub := queue.PublishOptions{Delay: d.Delay}

	return r.q.Publish(ctx, r.opts.Topic, body, pub)
}

// Start subscribes the runner to its topic and begins dispatching jobs
// to handlers. It returns immediately; the consumer runs in the
// background until Stop or the queue closes.
func (r *Runner) Start(ctx context.Context) error {
	r.subMu.Lock()
	defer r.subMu.Unlock()

	if r.sub != nil {
		return errors.New("jobs: already started")
	}

	sub, err := r.q.Subscribe(ctx, r.opts.Topic, r.dispatch)
	if err != nil {
		return err
	}

	r.sub = sub

	return nil
}

// Stop cancels the subscription and waits for in-flight jobs to finish
// or ctx to expire.
func (r *Runner) Stop(ctx context.Context) error {
	r.subMu.Lock()
	sub := r.sub
	r.sub = nil
	r.subMu.Unlock()

	if sub == nil {
		return nil
	}

	return sub.Close(ctx)
}

// dispatch is the queue.Handler that decodes envelopes and calls the
// registered handler for each job name. Retry, backoff, and DLQ are
// handled here.
func (r *Runner) dispatch(ctx context.Context, msg *queue.Message) error {
	var env Envelope
	if err := json.Unmarshal(msg.Body, &env); err != nil {
		hexlog.Error("jobs: bad envelope", "msg_id", msg.ID, "error", err)

		// Corrupted envelopes cannot be retried — send to DLQ if configured,
		// otherwise drop.
		r.deadLetter(ctx, msg, env, fmt.Errorf("envelope decode: %w", err))

		return nil // acknowledged: we can't recover
	}

	env.Attempt++

	h, ok := r.handlerFor(env.Name)
	if !ok {
		hexlog.Warn("jobs: unknown job", "name", env.Name, "msg_id", msg.ID)
		r.deadLetter(ctx, msg, env, fmt.Errorf("unknown job %q", env.Name))

		return nil
	}

	err := safeInvoke(ctx, h, env.Payload)
	if err == nil {
		return nil
	}

	max := env.MaxAttempts
	if max == 0 {
		max = r.opts.MaxAttempts
	}

	if max > 0 && env.Attempt >= max {
		hexlog.Error("jobs: exhausted retries", "name", env.Name, "attempt", env.Attempt, "error", err)
		r.deadLetter(ctx, msg, env, err)

		return nil // acknowledged; we handled the failure
	}

	// Republish with incremented attempt and backoff delay. env is copied
	// by value so we can reuse it directly.
	delay := backoff(env.Attempt, r.opts.BaseBackoff, r.opts.MaxBackoff)

	newBody, mErr := json.Marshal(env)
	if mErr != nil {
		// If we can't even re-serialise, DLQ it.
		hexlog.Error("jobs: re-marshal failed", "name", env.Name, "error", mErr)
		r.deadLetter(ctx, msg, env, mErr)

		return nil
	}

	if _, pErr := r.q.Publish(ctx, r.opts.Topic, newBody, queue.PublishOptions{Delay: delay}); pErr != nil {
		hexlog.Error("jobs: republish failed", "name", env.Name, "error", pErr)

		return pErr // let the queue redeliver the original
	}

	hexlog.Warn("jobs: retrying", "name", env.Name, "attempt", env.Attempt, "next_in", delay)

	return nil // acknowledged; retry is a fresh message
}

// handlerFor looks up a registered handler.
func (r *Runner) handlerFor(name string) (Handler, bool) {
	v, ok := r.handlers.Load(name)
	if !ok {
		return nil, false
	}

	h, ok := v.(Handler)

	return h, ok
}

// safeInvoke calls the handler with panic recovery so a bad handler does
// not crash the consumer.
func safeInvoke(ctx context.Context, h Handler, payload json.RawMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panic: %v", r)
		}
	}()

	return h(ctx, payload)
}

// deadLetter republishes env to the dead-letter topic. Errors here are
// logged but not propagated — the primary message has already been
// acknowledged.
func (r *Runner) deadLetter(ctx context.Context, msg *queue.Message, env Envelope, cause error) {
	if r.opts.DeadLetterTopic == "" {
		return
	}

	dlEnv := struct {
		Envelope
		OriginalID string `json:"original_id"`
		Error      string `json:"error"`
	}{
		Envelope:   env,
		OriginalID: msg.ID,
		Error:      cause.Error(),
	}

	body, err := json.Marshal(dlEnv)
	if err != nil {
		hexlog.Error("jobs: dead-letter marshal failed", "error", err)

		return
	}

	if _, err := r.q.Publish(ctx, r.opts.DeadLetterTopic, body); err != nil {
		hexlog.Error("jobs: dead-letter publish failed", "error", err)
	}
}

// backoff returns the delay for the given attempt number.
func backoff(attempt int, base, max time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	// base * 2^(attempt-1) — cap at max, guard against overflow.
	shift := attempt - 1
	if shift > 30 {
		return max
	}

	d := time.Duration(float64(base) * math.Pow(2, float64(shift)))
	if d < 0 || d > max {
		return max
	}

	return d
}
