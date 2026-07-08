package memory_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jordanbrauer/hex/cache"
	"github.com/jordanbrauer/hex/cache/memory"
)

func TestGet_missReturnsFalseNoError(t *testing.T) {
	c := memory.New()

	val, hit, err := c.Get(context.Background(), "nope")
	if err != nil {
		t.Errorf("Get miss error = %v, want nil", err)
	}

	if hit {
		t.Errorf("Get miss hit = true, want false")
	}

	if val != nil {
		t.Errorf("Get miss val = %v, want nil", val)
	}
}

func TestSetGet_roundTrip(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	if err := c.Set(ctx, "k", []byte("hello"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, hit, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !hit {
		t.Fatalf("Get hit = false, want true")
	}

	if string(val) != "hello" {
		t.Errorf("Get value = %q, want hello", val)
	}
}

func TestSet_defensiveCopyOnStore(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	buf := []byte("hello")
	_ = c.Set(ctx, "k", buf, 0)
	buf[0] = 'j' // mutate caller's slice

	val, _, _ := c.Get(ctx, "k")
	if string(val) != "hello" {
		t.Errorf("stored value mutated with caller's slice: got %q", val)
	}
}

func TestSet_defensiveCopyOnRead(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("hello"), 0)

	got1, _, _ := c.Get(ctx, "k")
	got1[0] = 'j' // mutate returned slice

	got2, _, _ := c.Get(ctx, "k")
	if string(got2) != "hello" {
		t.Errorf("stored value mutated via returned slice: got %q", got2)
	}
}

func TestSet_negativeTTL(t *testing.T) {
	c := memory.New()

	if err := c.Set(context.Background(), "k", []byte("v"), -time.Second); !errors.Is(err, cache.ErrNegativeTTL) {
		t.Errorf("Set(-1s) error = %v, want ErrNegativeTTL", err)
	}
}

func TestTTL_expiresEntry(t *testing.T) {
	now := time.Unix(0, 0)
	c := memory.WithClock(func() time.Time { return now })
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("v"), 100*time.Millisecond)

	// Still fresh.
	if _, hit, _ := c.Get(ctx, "k"); !hit {
		t.Fatalf("fresh entry missing")
	}

	// Advance past expiry.
	now = now.Add(101 * time.Millisecond)

	if _, hit, _ := c.Get(ctx, "k"); hit {
		t.Errorf("expired entry still present")
	}

	// Expired entry should have been lazily deleted.
	if c.Len() != 0 {
		t.Errorf("Len after expiry = %d, want 0", c.Len())
	}
}

func TestTTL_zeroMeansNoExpiry(t *testing.T) {
	now := time.Unix(0, 0)
	c := memory.WithClock(func() time.Time { return now })
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("v"), 0)

	now = now.Add(365 * 24 * time.Hour)

	if _, hit, _ := c.Get(ctx, "k"); !hit {
		t.Errorf("entry with ttl=0 expired after a year")
	}
}

func TestDelete(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("v"), 0)

	if err := c.Delete(ctx, "k"); err != nil {
		t.Errorf("Delete error = %v", err)
	}

	if _, hit, _ := c.Get(ctx, "k"); hit {
		t.Errorf("Delete did not remove entry")
	}

	// Deleting missing key is not an error.
	if err := c.Delete(ctx, "gone"); err != nil {
		t.Errorf("Delete(missing) error = %v", err)
	}
}

func TestHas(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("v"), 0)

	if ok, _ := c.Has(ctx, "k"); !ok {
		t.Errorf("Has(present) = false")
	}

	if ok, _ := c.Has(ctx, "nope"); ok {
		t.Errorf("Has(missing) = true")
	}
}

func TestClear(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	_ = c.Set(ctx, "a", []byte("1"), 0)
	_ = c.Set(ctx, "b", []byte("2"), 0)

	if err := c.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if c.Len() != 0 {
		t.Errorf("Len after Clear = %d, want 0", c.Len())
	}
}

func TestIncrement_freshKey(t *testing.T) {
	c := memory.New()

	got, err := c.Increment(context.Background(), "counter", 3)
	if err != nil {
		t.Fatalf("Increment: %v", err)
	}

	if got != 3 {
		t.Errorf("Increment fresh key = %d, want 3", got)
	}
}

func TestIncrement_accumulates(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	_, _ = c.Increment(ctx, "counter", 5)
	got, err := c.Increment(ctx, "counter", 2)

	if err != nil {
		t.Fatalf("Increment: %v", err)
	}

	if got != 7 {
		t.Errorf("Increment accumulated = %d, want 7", got)
	}
}

func TestIncrement_negativeDelta(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	_, _ = c.Increment(ctx, "counter", 10)
	got, _ := c.Increment(ctx, "counter", -3)

	if got != 7 {
		t.Errorf("Increment(-3) = %d, want 7", got)
	}
}

func TestIncrement_nonNumericFails(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	_ = c.Set(ctx, "text", []byte("hello"), 0)

	if _, err := c.Increment(ctx, "text", 1); err == nil {
		t.Errorf("Increment on non-numeric returned nil error")
	}
}

func TestIncrement_concurrentSafe(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, _ = c.Increment(ctx, "n", 1)
		}()
	}

	wg.Wait()

	final, _ := c.Increment(ctx, "n", 0)
	if final != 100 {
		t.Errorf("concurrent Increment total = %d, want 100", final)
	}
}

func TestGenericGet_typedRoundTrip(t *testing.T) {
	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	c := memory.New()
	ctx := context.Background()

	if err := cache.Set(ctx, c, "u", User{Name: "Ada", Age: 36}, time.Minute); err != nil {
		t.Fatalf("cache.Set: %v", err)
	}

	got, hit, err := cache.Get[User](ctx, c, "u")
	if err != nil {
		t.Fatalf("cache.Get: %v", err)
	}

	if !hit {
		t.Fatalf("miss on freshly Set entry")
	}

	if got.Name != "Ada" || got.Age != 36 {
		t.Errorf("cache.Get = %+v, want {Ada 36}", got)
	}
}

func TestGenericGet_missReturnsZero(t *testing.T) {
	c := memory.New()

	got, hit, err := cache.Get[int](context.Background(), c, "nope")
	if err != nil {
		t.Errorf("Get miss error = %v", err)
	}

	if hit {
		t.Errorf("Get miss hit = true")
	}

	if got != 0 {
		t.Errorf("Get miss got = %d, want 0", got)
	}
}

func TestRemember_callsFnOnMiss(t *testing.T) {
	c := memory.New()
	ctx := context.Background()

	var calls int64
	fn := func(context.Context) (string, error) {
		atomic.AddInt64(&calls, 1)

		return "computed", nil
	}

	got, err := cache.Remember(ctx, c, "k", time.Minute, fn)
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	if got != "computed" {
		t.Errorf("Remember = %q, want computed", got)
	}

	// Second call should hit the cache.
	got, err = cache.Remember(ctx, c, "k", time.Minute, fn)
	if err != nil {
		t.Fatalf("Remember 2: %v", err)
	}

	if got != "computed" {
		t.Errorf("Remember 2 = %q, want computed", got)
	}

	if atomic.LoadInt64(&calls) != 1 {
		t.Errorf("fn called %d times, want 1", calls)
	}
}

func TestRemember_propagatesFnError(t *testing.T) {
	c := memory.New()
	sentinel := errors.New("boom")

	_, err := cache.Remember(context.Background(), c, "k", time.Minute, func(context.Context) (int, error) {
		return 0, sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Errorf("Remember error = %v, want %v", err, sentinel)
	}

	if _, hit, _ := c.Get(context.Background(), "k"); hit {
		t.Errorf("Remember cached a value despite error")
	}
}
