// Package events provides a lightweight in-process publish/subscribe bus.
//
// A Bus routes named events to zero or more subscribers. Subscribers receive
// the event's variadic payload and may return an error; Emit returns the
// first non-nil error encountered while dispatching. EmitAsync fires and
// forgets, logging any handler errors.
package events

import (
	"log/slog"
	"slices"
	"sync"
)

// Subscriber is a handler invoked when an event is emitted. Its arguments are
// the payload passed to Emit; it returns an error to signal handler failure.
type Subscriber func(...any) error

// Bus is a concurrent event dispatcher. The zero value is not usable; call
// New.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string][]subscription
	nextID      uint64
}

type subscription struct {
	id uint64
	fn Subscriber
}

// New returns an empty Bus.
func New() *Bus {
	return &Bus{
		subscribers: make(map[string][]subscription),
	}
}

// On registers fn as a subscriber to event. It returns an unsubscribe function
// that removes fn from the bus when called. Calling the unsubscribe function
// more than once is safe.
func (b *Bus) On(event string, fn Subscriber) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID

	b.subscribers[event] = append(b.subscribers[event], subscription{id: id, fn: fn})

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		subs := b.subscribers[event]
		for i, s := range subs {
			if s.id == id {
				b.subscribers[event] = slices.Delete(subs, i, i+1)

				break
			}
		}

		if len(b.subscribers[event]) == 0 {
			delete(b.subscribers, event)
		}
	}
}

// Emit dispatches an event to every subscriber in registration order. It
// returns the first non-nil error a subscriber returns; remaining subscribers
// still run.
func (b *Bus) Emit(event string, data ...any) error {
	subs := b.snapshot(event)

	var firstErr error
	for _, s := range subs {
		if err := s.fn(data...); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// EmitAsync dispatches an event in a background goroutine. Subscriber errors
// are logged via log/slog; the caller does not block or observe them.
func (b *Bus) EmitAsync(event string, data ...any) {
	subs := b.snapshot(event)
	if len(subs) == 0 {
		return
	}

	go func() {
		for _, s := range subs {
			if err := s.fn(data...); err != nil {
				slog.Error("events: async subscriber returned error",
					"event", event,
					"error", err)
			}
		}
	}()
}

// Size returns the total number of subscriptions across all events.
func (b *Bus) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	total := 0
	for _, subs := range b.subscribers {
		total += len(subs)
	}

	return total
}

// snapshot returns a copy of the subscription list for event so callbacks can
// run without holding the bus lock and without racing against concurrent
// On/unsubscribe calls.
func (b *Bus) snapshot(event string) []subscription {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs, ok := b.subscribers[event]
	if !ok {
		return nil
	}

	return slices.Clone(subs)
}
