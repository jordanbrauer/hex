// Package queue defines a driver-agnostic message queue for cross-process,
// asynchronous delivery.
//
// The Queue interface is topic + byte-oriented so backends (memory, sqlite,
// SQS, RabbitMQ, Kafka) can all satisfy it. Higher-level structured jobs
// live in the hex/queue/jobs subpackage — see ADR-0009 for the layered
// design rationale.
//
// hex/queue is not the same as hex/events. Events are synchronous in-process
// pub/sub with no persistence, meant for lifecycle hooks and intra-process
// notifications. Queues are asynchronous, durable, and cross-process.
//
// Delivery semantics: at-least-once. Backends may deliver the same message
// more than once under failure; handlers must be idempotent.
package queue

import (
	"context"
	"errors"
	"time"
)

// ErrClosed is returned when a Queue method is called after Close.
var ErrClosed = errors.New("queue: closed")

// Message is a single item on a queue.
type Message struct {
	// ID uniquely identifies the message within its backend. Format is
	// backend-specific; consumers should treat it as opaque.
	ID string

	// Topic is the name of the topic this message was published to.
	Topic string

	// Body is the raw payload.
	Body []byte

	// Attempts is 1 on first delivery, incremented on redelivery.
	Attempts int

	// EnqueuedAt is the time the message was accepted by Publish.
	EnqueuedAt time.Time

	// DeliverAt is set when the message was scheduled with PublishOptions.
	// Delay. Zero means "immediate".
	DeliverAt time.Time

	// Metadata holds backend-specific fields (SQS ReceiptHandle, Kafka
	// partition/offset, etc.). Opaque to callers who target the interface;
	// backend-typed helpers may expose it.
	Metadata map[string]string
}

// Handler processes a Message. Returning nil acknowledges the message;
// returning an error signals redelivery per the backend's retry policy.
// Handlers must be safe for concurrent invocation.
type Handler func(ctx context.Context, msg *Message) error

// PublishOptions tunes a single Publish call.
type PublishOptions struct {
	// Delay defers delivery for at least this duration. Zero means
	// immediate. Backends without native delay support may error or
	// approximate (documented per backend).
	Delay time.Duration

	// DedupKey requests the backend deduplicate messages that carry the
	// same key within a backend-defined window. Empty means no
	// deduplication. Backends that do not support it silently ignore the
	// key.
	DedupKey string

	// Metadata is attached to the Message for consumers that need
	// contextual routing hints. Backend-specific keys pass through.
	Metadata map[string]string
}

// Queue is the driver-facing interface. Implementations must be safe for
// concurrent use.
type Queue interface {
	// Publish adds body to topic. Returns the assigned message ID.
	Publish(ctx context.Context, topic string, body []byte, opts ...PublishOptions) (id string, err error)

	// Subscribe starts a consumer that dispatches messages on topic to
	// handler. It returns immediately; the consumer runs in the
	// background until the returned Subscription is closed or the Queue
	// is closed. Multiple subscribers to the same topic share the
	// message stream (competing consumers) — each message is delivered
	// to exactly one subscriber (modulo redelivery).
	Subscribe(ctx context.Context, topic string, handler Handler) (Subscription, error)

	// Close stops all subscriptions and releases resources.
	Close(ctx context.Context) error
}

// Subscription is a live consumer registration. Close it to stop the
// consumer without touching other subscribers.
type Subscription interface {
	// Close stops the consumer. Returns after the current in-flight
	// handler (if any) finishes or ctx expires.
	Close(ctx context.Context) error

	// Topic returns the topic this subscription is bound to.
	Topic() string
}
