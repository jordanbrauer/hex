// Package container provides a type-safe IoC dependency injection container.
//
// A Container holds named bindings — factory functions that produce values on
// demand. Bindings are either transient (a fresh value per resolution) or
// singleton (a single value cached after the first successful resolution).
//
// The zero value of Container is not usable; call New.
package container

import (
	"fmt"
	"slices"
	"sync"
)

// Factory produces a value from a Container. It runs at resolution time and
// may fetch or construct dependencies from the same container.
type Factory func(*Container) (any, error)

// resolver is the subset of Container behavior needed by the generic helpers
// Make and Must. It exists so tests and consumers can substitute a fake
// container without depending on the concrete type.
type resolver interface {
	Make(string) (any, error)
}

// Container is an IoC dependency injection container. It is safe for
// concurrent use.
type Container struct {
	mu       sync.RWMutex
	bindings map[string]Factory
	onces    map[string]*singleton

	// resolving tracks the current in-progress resolution chain per goroutine
	// so that cyclic dependencies produce a clear error instead of a hang or
	// stack overflow.
	cycleMu   sync.Mutex
	resolving map[uint64][]string
}

// singleton wraps a factory with sync.Once so a singleton binding runs at most
// one time regardless of concurrent resolvers.
type singleton struct {
	once     sync.Once
	factory  Factory
	instance any
	err      error
}

// New returns an empty Container ready to accept bindings.
func New() *Container {
	return &Container{
		bindings:  make(map[string]Factory),
		onces:     make(map[string]*singleton),
		resolving: make(map[uint64][]string),
	}
}

// Bind registers a transient factory under name. Each call to Make invokes the
// factory again. If name is already bound, the previous binding is replaced.
func (c *Container) Bind(name string, fn Factory) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.bindings[name] = fn
	delete(c.onces, name)
}

// Singleton registers a factory under name that runs at most once. The
// returned value (or error) is cached and reused on every subsequent Make.
// Re-registering with Singleton or Bind clears the cached instance.
func (c *Container) Singleton(name string, fn Factory) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := &singleton{factory: fn}
	c.onces[name] = entry
	c.bindings[name] = func(inner *Container) (any, error) {
		entry.once.Do(func() {
			entry.instance, entry.err = entry.factory(inner)
		})

		return entry.instance, entry.err
	}
}

// Make resolves the binding named name and returns the produced value. Returns
// an error if the binding is not registered, the factory fails, or a cyclic
// resolution is detected.
func (c *Container) Make(name string) (any, error) {
	c.mu.RLock()
	fn, ok := c.bindings[name]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("container: binding %q not found", name)
	}

	gid := goid()

	c.cycleMu.Lock()
	chain := c.resolving[gid]
	if slices.Contains(chain, name) {
		cyclePath := append(slices.Clone(chain), name)
		c.cycleMu.Unlock()

		return nil, fmt.Errorf("container: cyclic dependency detected: %s", formatCycle(cyclePath))
	}
	c.resolving[gid] = append(chain, name)
	c.cycleMu.Unlock()

	defer func() {
		c.cycleMu.Lock()
		defer c.cycleMu.Unlock()

		current := c.resolving[gid]
		if len(current) <= 1 {
			delete(c.resolving, gid)
		} else {
			c.resolving[gid] = current[:len(current)-1]
		}
	}()

	return fn(c)
}

// Has reports whether a binding with the given name is registered.
func (c *Container) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.bindings[name]

	return ok
}

// Count returns the number of registered bindings.
func (c *Container) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.bindings)
}

// List returns the names of all registered bindings in alphabetical order.
func (c *Container) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.bindings))
	for k := range c.bindings {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	return keys
}

// Make resolves a binding by name and type-asserts the result to T. Returns an
// error if the binding is missing, the factory fails, or the produced value is
// not assignable to T.
func Make[T any](c resolver, name string) (T, error) {
	var zero T

	val, err := c.Make(name)
	if err != nil {
		return zero, err
	}

	resolved, ok := val.(T)
	if !ok {
		return zero, fmt.Errorf("container: binding %q is %T, not %T", name, val, zero)
	}

	return resolved, nil
}

// Must resolves a binding by name and type-asserts the result to T. It panics
// if the binding is missing, the factory fails, or the type does not match.
// Prefer Make in code that needs to handle errors gracefully; Must is intended
// for startup paths where a missing dependency is fatal by definition.
func Must[T any](c resolver, name string) T {
	resolved, err := Make[T](c, name)
	if err != nil {
		panic(err)
	}

	return resolved
}

func formatCycle(chain []string) string {
	out := ""
	for i, name := range chain {
		if i > 0 {
			out += " -> "
		}

		out += name
	}

	return out
}
