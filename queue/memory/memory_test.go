package memory_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/queue"
	"github.com/jordanbrauer/hex/queue/memory"
)

func TestPublishSubscribe_delivers(t *testing.T) {
	q := memory.New(memory.Options{})

	received := make(chan *queue.Message, 1)

	sub, err := q.Subscribe(context.Background(), "greetings", func(_ context.Context, m *queue.Message) error {
		received <- m

		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	defer sub.Close(context.Background())

	if _, err := q.Publish(context.Background(), "greetings", []byte("hello")); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case m := <-received:
		if string(m.Body) != "hello" {
			t.Errorf("body = %q, want hello", m.Body)
		}

		if m.Topic != "greetings" {
			t.Errorf("topic = %q, want greetings", m.Topic)
		}

		if m.Attempts != 1 {
			t.Errorf("attempts = %d, want 1", m.Attempts)
		}
	case <-time.After(time.Second):
		t.Fatal("no delivery within 1s")
	}

	_ = q.Close(context.Background())
}

func TestPublish_ordersFIFO(t *testing.T) {
	q := memory.New(memory.Options{})

	got := make([]string, 0, 5)

	var mu sync.Mutex

	done := make(chan struct{}, 5)

	sub, _ := q.Subscribe(context.Background(), "t", func(_ context.Context, m *queue.Message) error {
		mu.Lock()
		got = append(got, string(m.Body))
		mu.Unlock()

		done <- struct{}{}

		return nil
	})
	defer sub.Close(context.Background())

	for _, s := range []string{"a", "b", "c", "d", "e"} {
		if _, err := q.Publish(context.Background(), "t", []byte(s)); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 5; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}

	mu.Lock()
	defer mu.Unlock()

	for i, want := range []string{"a", "b", "c", "d", "e"} {
		if got[i] != want {
			t.Errorf("got[%d] = %q, want %q (full: %v)", i, got[i], want, got)
		}
	}

	_ = q.Close(context.Background())
}

func TestCompetingConsumers_splitStream(t *testing.T) {
	q := memory.New(memory.Options{})

	var (
		aCount int64
		bCount int64
	)

	subA, _ := q.Subscribe(context.Background(), "t", func(context.Context, *queue.Message) error {
		atomic.AddInt64(&aCount, 1)

		return nil
	})
	subB, _ := q.Subscribe(context.Background(), "t", func(context.Context, *queue.Message) error {
		atomic.AddInt64(&bCount, 1)

		return nil
	})

	defer subA.Close(context.Background())
	defer subB.Close(context.Background())

	const N = 40
	for i := 0; i < N; i++ {
		_, _ = q.Publish(context.Background(), "t", []byte("x"))
	}

	deadline := time.After(2 * time.Second)

	for atomic.LoadInt64(&aCount)+atomic.LoadInt64(&bCount) < N {
		select {
		case <-deadline:
			t.Fatalf("delivered %d+%d = %d/%d", aCount, bCount, aCount+bCount, N)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	total := atomic.LoadInt64(&aCount) + atomic.LoadInt64(&bCount)

	if total != N {
		t.Errorf("total delivered = %d, want %d", total, N)
	}

	// Both consumers should have received at least one — non-strict, but
	// with 40 messages and 2 subs the odds of a shutout are effectively nil.
	if atomic.LoadInt64(&aCount) == 0 || atomic.LoadInt64(&bCount) == 0 {
		t.Errorf("uneven split: a=%d b=%d", aCount, bCount)
	}

	_ = q.Close(context.Background())
}

func TestPublish_delayDefersDelivery(t *testing.T) {
	q := memory.New(memory.Options{PollInterval: 20 * time.Millisecond})

	received := make(chan time.Time, 1)

	sub, _ := q.Subscribe(context.Background(), "delayed", func(_ context.Context, m *queue.Message) error {
		received <- time.Now()

		return nil
	})
	defer sub.Close(context.Background())

	start := time.Now()

	if _, err := q.Publish(context.Background(), "delayed", []byte("x"), queue.PublishOptions{
		Delay: 150 * time.Millisecond,
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case got := <-received:
		if elapsed := got.Sub(start); elapsed < 100*time.Millisecond {
			t.Errorf("delivered after %v, want >= 100ms", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("delayed message never delivered")
	}

	_ = q.Close(context.Background())
}

func TestHandlerError_requeuesForRetry(t *testing.T) {
	q := memory.New(memory.Options{MaxRetries: 3})

	var attempts int64

	done := make(chan struct{})

	sub, _ := q.Subscribe(context.Background(), "flaky", func(context.Context, *queue.Message) error {
		n := atomic.AddInt64(&attempts, 1)
		if n < 3 {
			return errors.New("try again")
		}

		close(done)

		return nil
	})
	defer sub.Close(context.Background())

	_, _ = q.Publish(context.Background(), "flaky", []byte("x"))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("never succeeded, attempts=%d", attempts)
	}

	if got := atomic.LoadInt64(&attempts); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}

	_ = q.Close(context.Background())
}

func TestHandlerError_dropsAfterMaxRetries(t *testing.T) {
	q := memory.New(memory.Options{MaxRetries: 3})

	var attempts int64

	sub, _ := q.Subscribe(context.Background(), "bad", func(context.Context, *queue.Message) error {
		atomic.AddInt64(&attempts, 1)

		return errors.New("permanent")
	})
	defer sub.Close(context.Background())

	_, _ = q.Publish(context.Background(), "bad", []byte("x"))

	// Wait long enough that a fourth attempt would have happened.
	deadline := time.After(500 * time.Millisecond)

waitLoop:
	for atomic.LoadInt64(&attempts) < 3 {
		select {
		case <-deadline:
			break waitLoop
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	time.Sleep(200 * time.Millisecond)

	if got := atomic.LoadInt64(&attempts); got != 3 {
		t.Errorf("attempts = %d, want 3 (dropped after)", got)
	}

	if pending := q.PendingCount(); pending != 0 {
		t.Errorf("PendingCount = %d, want 0 after drop", pending)
	}

	_ = q.Close(context.Background())
}

func TestHandlerPanic_recovered(t *testing.T) {
	q := memory.New(memory.Options{MaxRetries: 2})

	var attempts int64

	done := make(chan struct{})

	sub, _ := q.Subscribe(context.Background(), "panicky", func(context.Context, *queue.Message) error {
		n := atomic.AddInt64(&attempts, 1)
		if n == 1 {
			panic("boom")
		}

		close(done)

		return nil
	})
	defer sub.Close(context.Background())

	_, _ = q.Publish(context.Background(), "panicky", []byte("x"))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("never recovered from panic")
	}

	_ = q.Close(context.Background())
}

func TestSubscribe_afterCloseErrors(t *testing.T) {
	q := memory.New(memory.Options{})
	_ = q.Close(context.Background())

	if _, err := q.Subscribe(context.Background(), "t", func(context.Context, *queue.Message) error { return nil }); !errors.Is(err, queue.ErrClosed) {
		t.Errorf("Subscribe after Close = %v, want ErrClosed", err)
	}
}

func TestPublish_afterCloseErrors(t *testing.T) {
	q := memory.New(memory.Options{})
	_ = q.Close(context.Background())

	if _, err := q.Publish(context.Background(), "t", []byte("x")); !errors.Is(err, queue.ErrClosed) {
		t.Errorf("Publish after Close = %v, want ErrClosed", err)
	}
}

func TestClose_stopsSubscribers(t *testing.T) {
	q := memory.New(memory.Options{})

	sub, _ := q.Subscribe(context.Background(), "t", func(context.Context, *queue.Message) error {
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := q.Close(ctx); err != nil {
		t.Errorf("Close: %v", err)
	}

	// Subscription's Close is now a no-op but must not hang.
	if err := sub.Close(ctx); err != nil {
		t.Errorf("sub.Close after Queue.Close: %v", err)
	}
}

func TestSubscription_closeStopsDeliveryToThisSub(t *testing.T) {
	q := memory.New(memory.Options{})

	var count int64

	sub, _ := q.Subscribe(context.Background(), "t", func(context.Context, *queue.Message) error {
		atomic.AddInt64(&count, 1)

		return nil
	})

	_, _ = q.Publish(context.Background(), "t", []byte("x"))

	// Wait for first delivery.
	deadline := time.After(time.Second)

	for atomic.LoadInt64(&count) == 0 {
		select {
		case <-deadline:
			t.Fatal("first delivery timeout")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	_ = sub.Close(context.Background())

	// Further publishes should not reach this sub.
	_, _ = q.Publish(context.Background(), "t", []byte("y"))

	time.Sleep(200 * time.Millisecond)

	if got := atomic.LoadInt64(&count); got != 1 {
		t.Errorf("count = %d, want 1 (no delivery after sub.Close)", got)
	}

	_ = q.Close(context.Background())
}

func TestPublish_defensiveCopy(t *testing.T) {
	q := memory.New(memory.Options{})

	got := make(chan []byte, 1)

	sub, _ := q.Subscribe(context.Background(), "t", func(_ context.Context, m *queue.Message) error {
		got <- m.Body

		return nil
	})
	defer sub.Close(context.Background())

	buf := []byte("hello")
	_, _ = q.Publish(context.Background(), "t", buf)
	buf[0] = 'j' // mutate caller's slice after publish

	select {
	case body := <-got:
		if string(body) != "hello" {
			t.Errorf("delivered %q, want hello (defensive copy failed)", body)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	_ = q.Close(context.Background())
}
