package container_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jordanbrauer/hex/container"
)

func TestNew_isEmpty(t *testing.T) {
	c := container.New()

	if got := c.Count(); got != 0 {
		t.Errorf("Count() = %d, want 0", got)
	}

	if c.Has("anything") {
		t.Errorf("Has() = true, want false on empty container")
	}
}

func TestBind_resolvesFactoryEachTime(t *testing.T) {
	c := container.New()
	calls := 0

	c.Bind("counter", func(*container.Container) (any, error) {
		calls++

		return calls, nil
	})

	for i := 1; i <= 3; i++ {
		got, err := c.Make("counter")
		if err != nil {
			t.Fatalf("Make() error = %v", err)
		}

		if got != i {
			t.Errorf("Make() = %v, want %d", got, i)
		}
	}
}

func TestSingleton_resolvesOnce(t *testing.T) {
	c := container.New()
	calls := 0

	c.Singleton("cached", func(*container.Container) (any, error) {
		calls++

		return calls, nil
	})

	for i := 0; i < 5; i++ {
		got, err := c.Make("cached")
		if err != nil {
			t.Fatalf("Make() error = %v", err)
		}

		if got != 1 {
			t.Errorf("Make() = %v, want 1 (cached)", got)
		}
	}

	if calls != 1 {
		t.Errorf("factory invoked %d times, want 1", calls)
	}
}

func TestSingleton_concurrentResolutionRunsFactoryOnce(t *testing.T) {
	c := container.New()

	var calls int64

	c.Singleton("cached", func(*container.Container) (any, error) {
		atomic.AddInt64(&calls, 1)

		return "value", nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if _, err := c.Make("cached"); err != nil {
				t.Errorf("Make() error = %v", err)
			}
		}()
	}

	wg.Wait()

	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Errorf("factory invoked %d times, want 1", got)
	}
}

func TestSingleton_rebindClearsCache(t *testing.T) {
	c := container.New()

	c.Singleton("v", func(*container.Container) (any, error) { return 1, nil })
	if got, _ := c.Make("v"); got != 1 {
		t.Fatalf("first resolution = %v, want 1", got)
	}

	c.Singleton("v", func(*container.Container) (any, error) { return 2, nil })
	if got, _ := c.Make("v"); got != 2 {
		t.Errorf("after rebind = %v, want 2", got)
	}
}

func TestMake_missingBindingReturnsError(t *testing.T) {
	c := container.New()

	_, err := c.Make("nope")
	if err == nil {
		t.Fatalf("Make(\"nope\") returned no error")
	}
}

func TestMake_factoryErrorIsReturned(t *testing.T) {
	c := container.New()
	sentinel := errors.New("boom")

	c.Bind("bad", func(*container.Container) (any, error) {
		return nil, sentinel
	})

	_, err := c.Make("bad")
	if !errors.Is(err, sentinel) {
		t.Errorf("Make() error = %v, want %v", err, sentinel)
	}
}

func TestMakeGeneric_typeAssertsResult(t *testing.T) {
	c := container.New()
	c.Bind("s", func(*container.Container) (any, error) { return "hello", nil })

	got, err := container.Make[string](c, "s")
	if err != nil {
		t.Fatalf("Make[string]() error = %v", err)
	}

	if got != "hello" {
		t.Errorf("Make[string]() = %q, want %q", got, "hello")
	}
}

func TestMakeGeneric_wrongTypeReturnsError(t *testing.T) {
	c := container.New()
	c.Bind("s", func(*container.Container) (any, error) { return "hello", nil })

	_, err := container.Make[int](c, "s")
	if err == nil {
		t.Errorf("Make[int]() returned no error for string binding")
	}
}

func TestMust_panicsOnError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Must did not panic on missing binding")
		}
	}()

	c := container.New()
	_ = container.Must[string](c, "missing")
}

func TestMust_returnsValue(t *testing.T) {
	c := container.New()
	c.Bind("x", func(*container.Container) (any, error) { return 42, nil })

	if got := container.Must[int](c, "x"); got != 42 {
		t.Errorf("Must[int]() = %d, want 42", got)
	}
}

func TestCycleDetection(t *testing.T) {
	c := container.New()

	c.Bind("a", func(inner *container.Container) (any, error) {
		return inner.Make("b")
	})
	c.Bind("b", func(inner *container.Container) (any, error) {
		return inner.Make("a")
	})

	_, err := c.Make("a")
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestNestedResolutionWorks(t *testing.T) {
	c := container.New()

	c.Bind("dep", func(*container.Container) (any, error) { return "hello", nil })
	c.Bind("wrapper", func(inner *container.Container) (any, error) {
		v, err := inner.Make("dep")
		if err != nil {
			return nil, err
		}

		return v.(string) + " world", nil
	})

	got, err := container.Make[string](c, "wrapper")
	if err != nil {
		t.Fatalf("Make error = %v", err)
	}

	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestHasCountList(t *testing.T) {
	c := container.New()
	c.Bind("b", func(*container.Container) (any, error) { return nil, nil })
	c.Bind("a", func(*container.Container) (any, error) { return nil, nil })
	c.Singleton("c", func(*container.Container) (any, error) { return nil, nil })

	if !c.Has("a") || !c.Has("b") || !c.Has("c") {
		t.Errorf("Has() missed a registered binding")
	}

	if c.Has("z") {
		t.Errorf("Has(\"z\") = true, want false")
	}

	if got := c.Count(); got != 3 {
		t.Errorf("Count() = %d, want 3", got)
	}

	list := c.List()
	want := []string{"a", "b", "c"}

	if len(list) != len(want) {
		t.Fatalf("List() len = %d, want %d (got %v)", len(list), len(want), list)
	}

	for i := range want {
		if list[i] != want[i] {
			t.Errorf("List()[%d] = %q, want %q", i, list[i], want[i])
		}
	}
}
