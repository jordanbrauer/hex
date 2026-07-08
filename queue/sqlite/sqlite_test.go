package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	hexdb "github.com/jordanbrauer/hex/db"
	"github.com/jordanbrauer/hex/queue"
	hexsqlite "github.com/jordanbrauer/hex/queue/sqlite"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()

	// SQLite :memory: is per-connection. Force a single pooled connection
	// so Init and subsequent Publish/Subscribe see the same database.
	db, err := hexdb.Open(context.Background(), hexdb.Config{
		Driver:       "sqlite",
		DSN:          ":memory:",
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := hexsqlite.Init(context.Background(), db); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	return db
}

func newQueue(t *testing.T, opts hexsqlite.Options) (*hexsqlite.Queue, func()) {
	t.Helper()

	if opts.PollInterval == 0 {
		opts.PollInterval = 20 * time.Millisecond
	}

	q := hexsqlite.New(openDB(t), opts)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		_ = q.Close(ctx)
	}

	return q, cleanup
}

func TestPublishSubscribe_roundTrip(t *testing.T) {
	q, cleanup := newQueue(t, hexsqlite.Options{})
	defer cleanup()

	got := make(chan string, 1)

	sub, err := q.Subscribe(context.Background(), "t", func(_ context.Context, m *queue.Message) error {
		got <- string(m.Body)

		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	defer sub.Close(context.Background())

	id, err := q.Publish(context.Background(), "t", []byte("hello"))
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if id == "" {
		t.Errorf("Publish returned empty id")
	}

	select {
	case body := <-got:
		if body != "hello" {
			t.Errorf("body = %q, want hello", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no delivery")
	}
}

func TestDurability_survivesReopen(t *testing.T) {
	// SQLite :memory: is per-connection, so we use a temp file.
	dir := t.TempDir()

	dsn := "file:" + dir + "/queue.db?_pragma=journal_mode(WAL)"

	db1, err := hexdb.Open(context.Background(), hexdb.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}

	if err := hexsqlite.Init(context.Background(), db1); err != nil {
		t.Fatalf("init: %v", err)
	}

	q1 := hexsqlite.New(db1, hexsqlite.Options{})

	if _, err := q1.Publish(context.Background(), "t", []byte("persistent")); err != nil {
		t.Fatalf("publish 1: %v", err)
	}

	_ = q1.Close(context.Background())
	_ = db1.Close()

	// Reopen.
	db2, err := hexdb.Open(context.Background(), hexdb.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}

	defer db2.Close()

	q2 := hexsqlite.New(db2, hexsqlite.Options{PollInterval: 20 * time.Millisecond})
	defer q2.Close(context.Background())

	got := make(chan string, 1)

	sub, _ := q2.Subscribe(context.Background(), "t", func(_ context.Context, m *queue.Message) error {
		got <- string(m.Body)

		return nil
	})

	defer sub.Close(context.Background())

	select {
	case body := <-got:
		if body != "persistent" {
			t.Errorf("body = %q, want persistent", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("persistent message not delivered after reopen")
	}
}

func TestDelay_defers(t *testing.T) {
	q, cleanup := newQueue(t, hexsqlite.Options{})
	defer cleanup()

	got := make(chan time.Time, 1)

	sub, _ := q.Subscribe(context.Background(), "delayed", func(context.Context, *queue.Message) error {
		got <- time.Now()

		return nil
	})

	defer sub.Close(context.Background())

	start := time.Now()

	if _, err := q.Publish(context.Background(), "delayed", []byte("x"), queue.PublishOptions{
		Delay: 200 * time.Millisecond,
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case t2 := <-got:
		if elapsed := t2.Sub(start); elapsed < 150*time.Millisecond {
			t.Errorf("delivered after %v, want >= 150ms", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("delayed never fired")
	}
}

func TestRetry_requeueOnError(t *testing.T) {
	q, cleanup := newQueue(t, hexsqlite.Options{MaxRetries: 3})
	defer cleanup()

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
	case <-time.After(3 * time.Second):
		t.Fatalf("attempts = %d", atomic.LoadInt64(&attempts))
	}
}

func TestRetry_drops(t *testing.T) {
	q, cleanup := newQueue(t, hexsqlite.Options{MaxRetries: 2})
	defer cleanup()

	var attempts int64

	sub, _ := q.Subscribe(context.Background(), "bad", func(context.Context, *queue.Message) error {
		atomic.AddInt64(&attempts, 1)

		return errors.New("permanent")
	})

	defer sub.Close(context.Background())

	_, _ = q.Publish(context.Background(), "bad", []byte("x"))

	// Wait for attempts to stabilise.
	time.Sleep(500 * time.Millisecond)

	if got := atomic.LoadInt64(&attempts); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
}

func TestDedup_sameKeyReturnsSameID(t *testing.T) {
	q, cleanup := newQueue(t, hexsqlite.Options{})
	defer cleanup()

	id1, err := q.Publish(context.Background(), "t", []byte("first"), queue.PublishOptions{
		DedupKey: "shared",
	})
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}

	id2, err := q.Publish(context.Background(), "t", []byte("second"), queue.PublishOptions{
		DedupKey: "shared",
	})
	if err != nil {
		t.Fatalf("second publish: %v", err)
	}

	if id1 != id2 {
		t.Errorf("dedup returned different ids: %q vs %q", id1, id2)
	}
}

func TestCompetingConsumers_noDoubleDelivery(t *testing.T) {
	// Two queues sharing the same DB → competing consumers on same topic.
	// Enable WAL mode + busy_timeout so concurrent writers don't hit
	// SQLITE_BUSY on the ~20 rapid publishes.
	dir := t.TempDir()
	dsn := "file:" + dir + "/queue.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"

	cfg := hexdb.Config{Driver: "sqlite", DSN: dsn, MaxOpenConns: 1}

	db1, _ := hexdb.Open(context.Background(), cfg)
	_ = hexsqlite.Init(context.Background(), db1)

	db2, _ := hexdb.Open(context.Background(), cfg)

	defer db1.Close()
	defer db2.Close()

	q1 := hexsqlite.New(db1, hexsqlite.Options{PollInterval: 10 * time.Millisecond})
	q2 := hexsqlite.New(db2, hexsqlite.Options{PollInterval: 10 * time.Millisecond})

	defer q1.Close(context.Background())
	defer q2.Close(context.Background())

	var (
		aCount int64
		bCount int64
	)

	subA, _ := q1.Subscribe(context.Background(), "t", func(context.Context, *queue.Message) error {
		atomic.AddInt64(&aCount, 1)

		return nil
	})
	subB, _ := q2.Subscribe(context.Background(), "t", func(context.Context, *queue.Message) error {
		atomic.AddInt64(&bCount, 1)

		return nil
	})

	defer subA.Close(context.Background())
	defer subB.Close(context.Background())

	const N = 20

	for i := 0; i < N; i++ {
		if _, err := q1.Publish(context.Background(), "t", []byte("x")); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	deadline := time.After(3 * time.Second)

	for atomic.LoadInt64(&aCount)+atomic.LoadInt64(&bCount) < N {
		select {
		case <-deadline:
			t.Fatalf("delivered %d + %d = %d/%d",
				aCount, bCount,
				atomic.LoadInt64(&aCount)+atomic.LoadInt64(&bCount), N)
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	total := atomic.LoadInt64(&aCount) + atomic.LoadInt64(&bCount)
	if total != N {
		t.Errorf("total = %d, want exactly %d (no double delivery)", total, N)
	}
}

func TestPublish_afterCloseErrors(t *testing.T) {
	q, _ := newQueue(t, hexsqlite.Options{})
	_ = q.Close(context.Background())

	if _, err := q.Publish(context.Background(), "t", []byte("x")); !errors.Is(err, queue.ErrClosed) {
		t.Errorf("Publish after Close = %v, want ErrClosed", err)
	}
}

func TestClose_stopsSubscribers(t *testing.T) {
	q, _ := newQueue(t, hexsqlite.Options{})

	var mu sync.Mutex

	var msgs []string

	sub, _ := q.Subscribe(context.Background(), "t", func(_ context.Context, m *queue.Message) error {
		mu.Lock()
		msgs = append(msgs, string(m.Body))
		mu.Unlock()

		return nil
	})

	_, _ = q.Publish(context.Background(), "t", []byte("first"))

	// Wait for delivery.
	deadline := time.After(2 * time.Second)

	for {
		mu.Lock()
		n := len(msgs)
		mu.Unlock()

		if n > 0 {
			break
		}

		select {
		case <-deadline:
			t.Fatal("first delivery never happened")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	if err := q.Close(context.Background()); err != nil {
		t.Errorf("Close: %v", err)
	}

	// After close, subscription is stopped.
	if err := sub.Close(context.Background()); err != nil {
		t.Errorf("sub.Close after Queue.Close: %v", err)
	}
}
