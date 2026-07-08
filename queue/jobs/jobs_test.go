package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/queue"
	"github.com/jordanbrauer/hex/queue/jobs"
	"github.com/jordanbrauer/hex/queue/memory"
)

func newRunner(t *testing.T, opts jobs.Options) (*jobs.Runner, *memory.Queue, func()) {
	t.Helper()

	q := memory.New(memory.Options{PollInterval: 20 * time.Millisecond})
	r := jobs.NewRunner(q, opts)

	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		_ = r.Stop(ctx)
		_ = q.Close(ctx)
	}

	return r, q, cleanup
}

func TestDispatch_deliversToRegisteredHandler(t *testing.T) {
	r, _, cleanup := newRunner(t, jobs.Options{})
	defer cleanup()

	type EmailPayload struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
	}

	got := make(chan EmailPayload, 1)

	r.Register("send-email", func(_ context.Context, payload json.RawMessage) error {
		var p EmailPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}

		got <- p

		return nil
	})

	_, err := r.Dispatch(context.Background(), "send-email", EmailPayload{
		To:      "user@example.com",
		Subject: "hello",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	select {
	case p := <-got:
		if p.To != "user@example.com" || p.Subject != "hello" {
			t.Errorf("payload = %+v, want To=user@example.com Subject=hello", p)
		}
	case <-time.After(time.Second):
		t.Fatal("no delivery")
	}
}

func TestDispatch_nameRequired(t *testing.T) {
	r, _, cleanup := newRunner(t, jobs.Options{})
	defer cleanup()

	if _, err := r.Dispatch(context.Background(), "", nil); err == nil {
		t.Errorf("empty name returned nil error")
	}
}

func TestRetry_exponentialBackoff(t *testing.T) {
	r, _, cleanup := newRunner(t, jobs.Options{
		MaxAttempts: 3,
		BaseBackoff: 50 * time.Millisecond,
		MaxBackoff:  time.Second,
	})
	defer cleanup()

	var (
		attempts int64
		done     = make(chan struct{})
	)

	r.Register("flaky", func(context.Context, json.RawMessage) error {
		n := atomic.AddInt64(&attempts, 1)
		if n < 3 {
			return errors.New("try again")
		}

		close(done)

		return nil
	})

	start := time.Now()
	_, _ = r.Dispatch(context.Background(), "flaky", nil)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("never succeeded, attempts=%d", atomic.LoadInt64(&attempts))
	}

	elapsed := time.Since(start)

	// Expected minimum: 50ms + 100ms = 150ms between attempts.
	if elapsed < 100*time.Millisecond {
		t.Errorf("succeeded in %v, want >= 100ms (backoff)", elapsed)
	}

	if got := atomic.LoadInt64(&attempts); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestRetry_maxAttemptsDropsWhenNoDLQ(t *testing.T) {
	r, _, cleanup := newRunner(t, jobs.Options{
		MaxAttempts: 3,
		BaseBackoff: 10 * time.Millisecond,
	})
	defer cleanup()

	var attempts int64

	r.Register("bad", func(context.Context, json.RawMessage) error {
		atomic.AddInt64(&attempts, 1)

		return errors.New("nope")
	})

	_, _ = r.Dispatch(context.Background(), "bad", nil)

	// Wait for attempts to plateau.
	deadline := time.After(2 * time.Second)

	for {
		select {
		case <-deadline:
			goto check
		default:
			if atomic.LoadInt64(&attempts) >= 3 {
				time.Sleep(300 * time.Millisecond) // give it a chance to retry once more

				goto check
			}

			time.Sleep(20 * time.Millisecond)
		}
	}

check:
	if got := atomic.LoadInt64(&attempts); got != 3 {
		t.Errorf("attempts = %d, want exactly 3", got)
	}
}

func TestDeadLetter_receivesExhaustedJobs(t *testing.T) {
	q := memory.New(memory.Options{PollInterval: 20 * time.Millisecond})

	r := jobs.NewRunner(q, jobs.Options{
		MaxAttempts:     2,
		BaseBackoff:     10 * time.Millisecond,
		DeadLetterTopic: "jobs.dlq",
	})

	_ = r.Start(context.Background())

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		_ = r.Stop(ctx)
		_ = q.Close(ctx)
	}()

	// Consumer on DLQ to observe.
	dlq := make(chan *queue.Message, 1)
	dlqSub, _ := q.Subscribe(context.Background(), "jobs.dlq", func(_ context.Context, m *queue.Message) error {
		dlq <- m

		return nil
	})

	defer dlqSub.Close(context.Background())

	r.Register("always-fails", func(context.Context, json.RawMessage) error {
		return errors.New("permanent")
	})

	_, _ = r.Dispatch(context.Background(), "always-fails", map[string]string{"key": "value"})

	select {
	case m := <-dlq:
		var payload struct {
			Name       string          `json:"name"`
			Payload    json.RawMessage `json:"payload"`
			OriginalID string          `json:"original_id"`
			Error      string          `json:"error"`
			Attempt    int             `json:"attempt"`
		}

		if err := json.Unmarshal(m.Body, &payload); err != nil {
			t.Fatalf("dlq unmarshal: %v", err)
		}

		if payload.Name != "always-fails" {
			t.Errorf("dlq name = %q, want always-fails", payload.Name)
		}

		if payload.Attempt < 2 {
			t.Errorf("dlq attempt = %d, want >= 2", payload.Attempt)
		}

		if payload.Error == "" {
			t.Errorf("dlq missing error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("dlq never received message")
	}
}

func TestDeadLetter_unknownJobNameGoesToDLQ(t *testing.T) {
	q := memory.New(memory.Options{PollInterval: 20 * time.Millisecond})

	r := jobs.NewRunner(q, jobs.Options{
		DeadLetterTopic: "jobs.dlq",
	})

	_ = r.Start(context.Background())

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		_ = r.Stop(ctx)
		_ = q.Close(ctx)
	}()

	dlq := make(chan *queue.Message, 1)
	dlqSub, _ := q.Subscribe(context.Background(), "jobs.dlq", func(_ context.Context, m *queue.Message) error {
		dlq <- m

		return nil
	})

	defer dlqSub.Close(context.Background())

	// Dispatch a job that no handler is registered for.
	_, _ = r.Dispatch(context.Background(), "orphan", nil)

	select {
	case <-dlq:
	case <-time.After(2 * time.Second):
		t.Fatal("unknown job did not reach DLQ")
	}
}

func TestDispatch_delayDefersExecution(t *testing.T) {
	r, _, cleanup := newRunner(t, jobs.Options{})
	defer cleanup()

	var ranAt time.Time

	var mu sync.Mutex

	done := make(chan struct{})

	r.Register("delayed", func(context.Context, json.RawMessage) error {
		mu.Lock()
		ranAt = time.Now()
		mu.Unlock()

		close(done)

		return nil
	})

	start := time.Now()

	_, err := r.Dispatch(context.Background(), "delayed", nil, jobs.DispatchOptions{
		Delay: 150 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("delayed job never ran")
	}

	mu.Lock()
	defer mu.Unlock()

	if elapsed := ranAt.Sub(start); elapsed < 100*time.Millisecond {
		t.Errorf("ran after %v, want >= 100ms", elapsed)
	}
}

func TestJobPanic_recovered(t *testing.T) {
	r, _, cleanup := newRunner(t, jobs.Options{
		MaxAttempts: 2,
		BaseBackoff: 10 * time.Millisecond,
	})
	defer cleanup()

	var (
		attempts int64
		done     = make(chan struct{})
	)

	r.Register("panicky", func(context.Context, json.RawMessage) error {
		n := atomic.AddInt64(&attempts, 1)
		if n == 1 {
			panic("boom")
		}

		close(done)

		return nil
	})

	_, _ = r.Dispatch(context.Background(), "panicky", nil)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("never recovered from panic")
	}
}

func TestPerJobMaxAttempts_overridesRunnerDefault(t *testing.T) {
	q := memory.New(memory.Options{PollInterval: 10 * time.Millisecond})

	r := jobs.NewRunner(q, jobs.Options{
		MaxAttempts:     3,
		BaseBackoff:     10 * time.Millisecond,
		DeadLetterTopic: "dlq",
	})

	_ = r.Start(context.Background())

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		_ = r.Stop(ctx)
		_ = q.Close(ctx)
	}()

	var attempts int64

	r.Register("job", func(context.Context, json.RawMessage) error {
		atomic.AddInt64(&attempts, 1)

		return errors.New("fail")
	})

	dlq := make(chan struct{}, 1)
	dlqSub, _ := q.Subscribe(context.Background(), "dlq", func(context.Context, *queue.Message) error {
		dlq <- struct{}{}

		return nil
	})

	defer dlqSub.Close(context.Background())

	// Override MaxAttempts to 1 for this dispatch.
	_, _ = r.Dispatch(context.Background(), "job", nil, jobs.DispatchOptions{
		MaxAttempts: 1,
	})

	select {
	case <-dlq:
	case <-time.After(2 * time.Second):
		t.Fatal("expected DLQ delivery after 1 attempt")
	}

	if got := atomic.LoadInt64(&attempts); got != 1 {
		t.Errorf("attempts = %d, want 1 (per-job override)", got)
	}
}

func TestStart_twiceErrors(t *testing.T) {
	q := memory.New(memory.Options{})

	defer q.Close(context.Background())

	r := jobs.NewRunner(q, jobs.Options{})

	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := r.Start(context.Background()); err == nil {
		t.Errorf("second Start returned nil error")
	}

	_ = r.Stop(context.Background())
}
