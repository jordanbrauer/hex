// Package memory is an in-process queue backend. It is intended for tests
// and single-process applications that need queue semantics without
// persistent infrastructure.
//
// Delivery semantics: at-least-once (a handler that panics or returns an
// error causes redelivery). Ordering: FIFO per topic. Concurrency:
// competing consumers on the same topic split the message stream.
package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jordanbrauer/hex/queue"
)

// Options configures a memory Queue.
type Options struct {
	// MaxRetries caps redelivery attempts before a message is dropped.
	// Zero means unlimited (retry forever). Callers that need dead-letter
	// routing should use the hex/queue/jobs layer.
	MaxRetries int

	// RetryDelay is the wait between redeliveries of the same message.
	// Zero means immediate.
	RetryDelay time.Duration

	// PollInterval controls how often the background scheduler looks for
	// due delayed messages. Defaults to 50ms.
	PollInterval time.Duration
}

// Queue is an in-memory queue.Queue implementation.
type Queue struct {
	mu     sync.Mutex
	topics map[string]*topic
	closed bool
	seq    atomic.Uint64
	opts   Options

	// ctx is cancelled on Close so background pollers wind down.
	ctx    context.Context
	cancel context.CancelFunc
}

// New returns an empty in-memory queue.
func New(opts Options) *Queue {
	if opts.PollInterval == 0 {
		opts.PollInterval = 50 * time.Millisecond
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Queue{
		topics: make(map[string]*topic),
		opts:   opts,
		ctx:    ctx,
		cancel: cancel,
	}
}

// -- topic bookkeeping ----------------------------------------------------

type topic struct {
	name string

	mu      sync.Mutex
	ready   []*queue.Message // FIFO of messages ready for delivery
	delayed []*delayedMsg    // heap-ish list; scanned on tick
	subs    []*sub           // active subscribers
	nextIdx int              // round-robin index across subs
	notify  chan struct{}    // signal delivery loops
}

type delayedMsg struct {
	msg *queue.Message
}

type sub struct {
	id      uint64
	topic   *topic
	handler queue.Handler
	ctx     context.Context
	cancel  context.CancelFunc
	q       *Queue
	done    chan struct{} // closed when the goroutine exits
	// dispatcher hands messages to this sub here. Buffered with cap 1
	// so the round-robin dispatcher can queue a message on a sub while
	// that sub is still processing its previous message — without the
	// buffer, only subs that happen to be at `<-msgCh` at the moment
	// dispatchReady iterates receive anything, and fast handlers like
	// `atomic.AddInt64` can produce total shutouts of other subs on
	// single-CPU CI runners.
	msgCh    chan *queue.Message
	inFlight sync.WaitGroup
}

func (q *Queue) getOrCreateTopic(name string) *topic {
	q.mu.Lock()
	defer q.mu.Unlock()

	if t, ok := q.topics[name]; ok {
		return t
	}

	t := &topic{
		name:   name,
		notify: make(chan struct{}, 1),
	}
	q.topics[name] = t

	// Kick off a scheduler for this topic to drain the delayed queue,
	// and a dispatcher to hand ready messages to subscribers fairly.
	go q.scheduler(t)
	go q.dispatcher(t)

	return t
}

// scheduler moves due delayed messages into `ready` and wakes deliverers.
func (q *Queue) scheduler(t *topic) {
	ticker := time.NewTicker(q.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
			t.promoteDue()
		}
	}
}

// dispatcher is the single goroutine responsible for handing ready
// messages to subscribers in round-robin order. Using one dispatcher
// per topic (instead of letting each sub's goroutine race to pull off
// a shared FIFO) guarantees a fair split across competing consumers —
// without it, OS scheduling can let one sub's goroutine claim many
// messages in a row before another ever runs, especially on CI
// runners with few CPUs.
func (q *Queue) dispatcher(t *topic) {
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-t.notify:
			t.dispatchReady()
		}
	}
}

// dispatchReady hands off as many ready messages as current
// subscriber availability allows, advancing the round-robin cursor
// one step per successful hand-off. Stops (without error) once no
// subscriber can currently accept a message; the next wake() call
// (triggered by a sub finishing its current message, a new publish,
// or a promoted delayed message) resumes dispatch.
func (t *topic) dispatchReady() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for len(t.ready) > 0 && len(t.subs) > 0 {
		n := len(t.subs)
		delivered := false

		for i := 0; i < n; i++ {
			idx := (t.nextIdx + i) % n
			s := t.subs[idx]

			select {
			case s.msgCh <- t.ready[0]:
				t.nextIdx = (idx + 1) % n
				delivered = true
			default:
				continue
			}

			break
		}

		if !delivered {
			return
		}

		t.ready = t.ready[1:]
	}
}

// promoteDue moves any delayed messages whose DeliverAt has passed into
// the ready FIFO and signals subscribers.
func (t *topic) promoteDue() {
	t.mu.Lock()

	now := time.Now()
	remaining := t.delayed[:0]
	promoted := false

	for _, d := range t.delayed {
		if !d.msg.DeliverAt.After(now) {
			t.ready = append(t.ready, d.msg)
			promoted = true
		} else {
			remaining = append(remaining, d)
		}
	}

	t.delayed = remaining
	t.mu.Unlock()

	if promoted {
		t.wake()
	}
}

// wake tries to signal without blocking.
func (t *topic) wake() {
	select {
	case t.notify <- struct{}{}:
	default:
	}
}

// -- Publish --------------------------------------------------------------

// Publish adds body to topic.
func (q *Queue) Publish(ctx context.Context, topicName string, body []byte, opts ...queue.PublishOptions) (string, error) {
	q.mu.Lock()

	if q.closed {
		q.mu.Unlock()

		return "", queue.ErrClosed
	}

	q.mu.Unlock()

	var o queue.PublishOptions
	if len(opts) > 0 {
		o = opts[0]
	}

	id := fmt.Sprintf("m%d", q.seq.Add(1))
	now := time.Now()

	deliverAt := now
	if o.Delay > 0 {
		deliverAt = now.Add(o.Delay)
	}

	// Defensive copy so caller-side mutations don't reach handlers.
	bodyCopy := make([]byte, len(body))
	copy(bodyCopy, body)

	// Copy metadata to avoid shared-map surprises.
	var meta map[string]string
	if len(o.Metadata) > 0 {
		meta = make(map[string]string, len(o.Metadata))
		for k, v := range o.Metadata {
			meta[k] = v
		}
	}

	msg := &queue.Message{
		ID:         id,
		Topic:      topicName,
		Body:       bodyCopy,
		Attempts:   0,
		EnqueuedAt: now,
		DeliverAt:  deliverAt,
		Metadata:   meta,
	}

	t := q.getOrCreateTopic(topicName)

	t.mu.Lock()
	if o.Delay > 0 {
		t.delayed = append(t.delayed, &delayedMsg{msg: msg})
	} else {
		t.ready = append(t.ready, msg)
	}
	t.mu.Unlock()

	t.wake()

	return id, nil
}

// -- Subscribe ------------------------------------------------------------

// Subscribe attaches a handler to a topic. Multiple subscribers compete
// for messages (each message is delivered to exactly one).
func (q *Queue) Subscribe(ctx context.Context, topicName string, handler queue.Handler) (queue.Subscription, error) {
	q.mu.Lock()

	if q.closed {
		q.mu.Unlock()

		return nil, queue.ErrClosed
	}

	q.mu.Unlock()

	if handler == nil {
		return nil, errors.New("queue/memory: nil handler")
	}

	t := q.getOrCreateTopic(topicName)

	subCtx, cancel := context.WithCancel(q.ctx)

	s := &sub{
		id:      q.seq.Add(1),
		topic:   t,
		handler: handler,
		ctx:     subCtx,
		cancel:  cancel,
		q:       q,
		done:    make(chan struct{}),
		msgCh:   make(chan *queue.Message, 1),
	}

	t.mu.Lock()
	t.subs = append(t.subs, s)
	t.mu.Unlock()

	go s.run()

	// Nudge in case there are already ready messages.
	t.wake()

	return s, nil
}

// -- delivery loop --------------------------------------------------------

// run receives messages the topic's dispatcher hands to this sub (in
// round-robin turn with any other subs on the same topic) and invokes
// the handler. Messages that error are requeued (bounded by
// MaxRetries) or dropped.
func (s *sub) run() {
	defer close(s.done)

	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-s.msgCh:
			s.inFlight.Add(1)
			s.deliver(msg)
			s.inFlight.Done()

			// We may now be able to accept another message — nudge the
			// dispatcher in case there's a backlog waiting on us.
			s.topic.wake()

			if s.ctx.Err() != nil {
				return
			}
		}
	}
}

// deliver runs the handler with panic recovery. On error/panic the
// message is requeued unless MaxRetries is exceeded.
func (s *sub) deliver(msg *queue.Message) {
	msg.Attempts++

	err := s.callHandler(msg)
	if err == nil {
		return
	}

	if s.q.opts.MaxRetries > 0 && msg.Attempts >= s.q.opts.MaxRetries {
		return // drop
	}

	// Requeue after RetryDelay (if any).
	if s.q.opts.RetryDelay > 0 {
		msg.DeliverAt = time.Now().Add(s.q.opts.RetryDelay)

		s.topic.mu.Lock()
		s.topic.delayed = append(s.topic.delayed, &delayedMsg{msg: msg})
		s.topic.mu.Unlock()

		return
	}

	s.topic.mu.Lock()
	s.topic.ready = append(s.topic.ready, msg)
	s.topic.mu.Unlock()

	s.topic.wake()
}

// callHandler wraps the user handler in panic recovery so a bad handler
// does not crash the consumer goroutine.
func (s *sub) callHandler(msg *queue.Message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()

	return s.handler(s.ctx, msg)
}

// -- Subscription ---------------------------------------------------------

// Close stops the subscription.
func (s *sub) Close(ctx context.Context) error {
	s.cancel()

	// Detach from topic.
	s.topic.mu.Lock()

	for i, other := range s.topic.subs {
		if other == s {
			s.topic.subs = append(s.topic.subs[:i], s.topic.subs[i+1:]...)

			break
		}
	}

	s.topic.mu.Unlock()

	// Wait for the delivery goroutine to exit or ctx to expire.
	select {
	case <-s.done:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Wait for any in-flight handler.
	waitCh := make(chan struct{})

	go func() {
		s.inFlight.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (s *sub) Topic() string { return s.topic.name }

// -- Queue close ----------------------------------------------------------

// Close stops all consumers and drops any pending messages.
func (q *Queue) Close(ctx context.Context) error {
	q.mu.Lock()

	if q.closed {
		q.mu.Unlock()

		return nil
	}

	q.closed = true
	q.mu.Unlock()

	q.cancel()

	// Close every subscription.
	q.mu.Lock()

	var subs []*sub
	for _, t := range q.topics {
		t.mu.Lock()
		subs = append(subs, t.subs...)
		t.subs = nil
		t.mu.Unlock()
	}

	q.mu.Unlock()

	for _, s := range subs {
		if err := s.Close(ctx); err != nil {
			return err
		}
	}

	return nil
}

// PendingCount returns the number of ready messages across all topics.
// Useful for tests; not part of the queue.Queue interface.
func (q *Queue) PendingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	total := 0

	for _, t := range q.topics {
		t.mu.Lock()
		total += len(t.ready) + len(t.delayed)
		t.mu.Unlock()
	}

	return total
}

// compile-time proof
var _ queue.Queue = (*Queue)(nil)
