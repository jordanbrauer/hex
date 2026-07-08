package events_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/events"
)

func TestNew_isEmpty(t *testing.T) {
	if got := events.New().Size(); got != 0 {
		t.Errorf("Size() = %d, want 0", got)
	}
}

func TestOn_incrementsSize(t *testing.T) {
	b := events.New()
	b.On("foo", func(...any) error { return nil })
	b.On("foo", func(...any) error { return nil })
	b.On("bar", func(...any) error { return nil })

	if got := b.Size(); got != 3 {
		t.Errorf("Size() = %d, want 3", got)
	}
}

func TestEmit_deliversInOrder(t *testing.T) {
	b := events.New()
	var got []int
	var mu sync.Mutex

	for i := 1; i <= 3; i++ {
		i := i
		b.On("x", func(...any) error {
			mu.Lock()
			defer mu.Unlock()

			got = append(got, i)

			return nil
		})
	}

	if err := b.Emit("x"); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("dispatch order = %v, want [1 2 3]", got)
	}
}

func TestEmit_passesPayload(t *testing.T) {
	b := events.New()
	var seen []any

	b.On("x", func(data ...any) error {
		seen = data

		return nil
	})

	if err := b.Emit("x", "hello", 42); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	if len(seen) != 2 || seen[0] != "hello" || seen[1] != 42 {
		t.Errorf("subscriber payload = %v, want [hello 42]", seen)
	}
}

func TestEmit_unknownEventIsNoOp(t *testing.T) {
	if err := events.New().Emit("nope"); err != nil {
		t.Errorf("Emit(\"nope\") error = %v, want nil", err)
	}
}

func TestEmit_returnsFirstError(t *testing.T) {
	b := events.New()
	first := errors.New("first")
	second := errors.New("second")

	b.On("x", func(...any) error { return first })
	b.On("x", func(...any) error { return second })

	err := b.Emit("x")
	if !errors.Is(err, first) {
		t.Errorf("Emit() = %v, want first", err)
	}
}

func TestOn_unsubscribeRemovesHandler(t *testing.T) {
	b := events.New()
	var calls int64

	off := b.On("x", func(...any) error {
		atomic.AddInt64(&calls, 1)

		return nil
	})

	_ = b.Emit("x")
	off()
	_ = b.Emit("x")

	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Errorf("handler invoked %d times after unsubscribe, want 1", got)
	}

	if got := b.Size(); got != 0 {
		t.Errorf("Size() = %d after unsubscribe, want 0", got)
	}

	// Second call is safe.
	off()
}

func TestOn_unsubscribeOnlyRemovesTargetedHandler(t *testing.T) {
	b := events.New()

	var aCalls, bCalls int64
	offA := b.On("x", func(...any) error {
		atomic.AddInt64(&aCalls, 1)

		return nil
	})
	b.On("x", func(...any) error {
		atomic.AddInt64(&bCalls, 1)

		return nil
	})

	offA()
	_ = b.Emit("x")

	if got := atomic.LoadInt64(&aCalls); got != 0 {
		t.Errorf("A invoked %d times, want 0", got)
	}

	if got := atomic.LoadInt64(&bCalls); got != 1 {
		t.Errorf("B invoked %d times, want 1", got)
	}
}

func TestEmitAsync_deliversAndDoesNotBlock(t *testing.T) {
	b := events.New()
	done := make(chan struct{})

	b.On("x", func(...any) error {
		close(done)

		return nil
	})

	b.EmitAsync("x")

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("EmitAsync did not deliver within 1s")
	}
}

func TestEmit_snapshotProtectsAgainstConcurrentUnsubscribe(t *testing.T) {
	b := events.New()

	off := b.On("x", func(...any) error {
		return nil
	})

	// Emit while a concurrent unsubscribe races — no panic expected because
	// Emit takes a snapshot of the subscriber slice.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)

		go func() { defer wg.Done(); _ = b.Emit("x") }()
		go func() { defer wg.Done(); off() }()
	}

	wg.Wait()
}
