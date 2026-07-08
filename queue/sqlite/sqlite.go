// Package sqlite is a durable, single-node queue backend built on
// database/sql + SQLite. Messages survive process restart. Delivery is
// at-least-once: a subscriber claims a message, and only DELETEs it
// after the handler returns nil.
//
// Consumers must blank-import their SQLite driver (typically
// modernc.org/sqlite). This package uses only database/sql.
//
// The schema lives in schema.sql (embedded). Call Init once on a fresh
// database to create the tables.
//
// # SQLite pool notes
//
// SQLite has two footguns that hex/queue/sqlite consumers should know about:
//
//   - :memory: databases are per-connection. If the *sql.DB pool holds more
//     than one connection, Init runs against one and Publish/Subscribe may
//     land on another that has no schema. Use MaxOpenConns: 1 with :memory:.
//   - File-backed databases with concurrent writers hit SQLITE_BUSY unless
//     WAL mode and a busy_timeout are enabled. Configure the DSN accordingly:
//     "file:queue.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)".
package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jordanbrauer/hex/queue"
)

//go:embed schema.sql
var schemaSQL string

// Options tune a sqlite Queue.
type Options struct {
	// PollInterval is how often subscribers scan for new messages.
	// Defaults to 200ms. Tune down for lower latency, up for lower load.
	PollInterval time.Duration

	// MaxRetries caps redelivery attempts. Zero means unlimited. Callers
	// wanting DLQ should use the hex/queue/jobs layer.
	MaxRetries int

	// RetryDelay is the wait before a failed message is redelivered.
	// Zero means immediate.
	RetryDelay time.Duration

	// VisibilityTimeout is how long a claimed message stays hidden from
	// other consumers before being made visible again (assumed
	// crashed). Defaults to 30 seconds.
	VisibilityTimeout time.Duration
}

// Queue is the sqlite-backed implementation of queue.Queue.
type Queue struct {
	db     *sql.DB
	opts   Options
	closed atomic.Bool

	subMu sync.Mutex
	subs  []*subscription

	// consumerID uniquely identifies this Queue instance in claimed_by so
	// visibility recovery does not steal our own in-flight messages.
	consumerID string
}

// Init runs the schema DDL. Safe to call more than once (uses IF NOT EXISTS).
// Consumers that also use hex/db/sqlite for their own migrations can skip
// this and include schema.sql content in their migration set.
func Init(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("queue/sqlite: init schema: %w", err)
	}

	return nil
}

// New returns a sqlite Queue backed by db. The database must already
// contain the queue schema (call Init).
func New(db *sql.DB, opts Options) *Queue {
	if opts.PollInterval == 0 {
		opts.PollInterval = 200 * time.Millisecond
	}

	if opts.VisibilityTimeout == 0 {
		opts.VisibilityTimeout = 30 * time.Second
	}

	// Random consumer ID so parallel processes don't collide.
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)

	return &Queue{
		db:         db,
		opts:       opts,
		consumerID: hex.EncodeToString(buf),
	}
}

// Publish inserts a message.
func (q *Queue) Publish(ctx context.Context, topic string, body []byte, opts ...queue.PublishOptions) (string, error) {
	if q.closed.Load() {
		return "", queue.ErrClosed
	}

	var o queue.PublishOptions
	if len(opts) > 0 {
		o = opts[0]
	}

	nowMs := time.Now().UnixMilli()

	deliverMs := nowMs
	if o.Delay > 0 {
		deliverMs = nowMs + o.Delay.Milliseconds()
	}

	var metaJSON sql.NullString

	if len(o.Metadata) > 0 {
		b, err := json.Marshal(o.Metadata)
		if err != nil {
			return "", fmt.Errorf("queue/sqlite: marshal metadata: %w", err)
		}

		metaJSON = sql.NullString{String: string(b), Valid: true}
	}

	var dedup sql.NullString
	if o.DedupKey != "" {
		dedup = sql.NullString{String: o.DedupKey, Valid: true}
	}

	res, err := q.db.ExecContext(ctx,
		`INSERT INTO queue_messages (topic, body, enqueued_at, deliver_at, dedup_key, metadata)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		topic, body, nowMs, deliverMs, dedup, metaJSON,
	)
	if err != nil {
		// Best-effort dedup: unique-constraint failures return the existing
		// message id. Other errors bubble up.
		if o.DedupKey != "" {
			var id int64
			row := q.db.QueryRowContext(ctx,
				`SELECT id FROM queue_messages WHERE topic = ? AND dedup_key = ?`,
				topic, o.DedupKey,
			)

			if scanErr := row.Scan(&id); scanErr == nil {
				return strconv.FormatInt(id, 10), nil
			}
		}

		return "", fmt.Errorf("queue/sqlite: publish: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(id, 10), nil
}

// Subscribe starts a poller that dispatches messages on topic to handler.
func (q *Queue) Subscribe(ctx context.Context, topic string, handler queue.Handler) (queue.Subscription, error) {
	if q.closed.Load() {
		return nil, queue.ErrClosed
	}

	if handler == nil {
		return nil, errors.New("queue/sqlite: nil handler")
	}

	subCtx, cancel := context.WithCancel(ctx)

	s := &subscription{
		q:       q,
		topic:   topic,
		handler: handler,
		ctx:     subCtx,
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	q.subMu.Lock()
	q.subs = append(q.subs, s)
	q.subMu.Unlock()

	go s.run()

	return s, nil
}

// Close stops all subscribers and closes the underlying DB is left to
// the caller (hex/db semantics — caller owns *sql.DB).
func (q *Queue) Close(ctx context.Context) error {
	if !q.closed.CompareAndSwap(false, true) {
		return nil
	}

	q.subMu.Lock()
	subs := make([]*subscription, len(q.subs))
	copy(subs, q.subs)
	q.subs = nil
	q.subMu.Unlock()

	for _, s := range subs {
		if err := s.Close(ctx); err != nil {
			return err
		}
	}

	return nil
}

// -- subscription ----------------------------------------------------------

type subscription struct {
	q       *Queue
	topic   string
	handler queue.Handler
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
}

func (s *subscription) Topic() string { return s.topic }

func (s *subscription) Close(ctx context.Context) error {
	s.cancel()

	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *subscription) run() {
	defer close(s.done)

	ticker := time.NewTicker(s.q.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		msg, err := s.claim()
		if err != nil {
			// Log-and-back-off — errors here are typically transient
			// (locked DB, dropped connection). Poll again next tick.
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
			}

			continue
		}

		if msg == nil {
			// No ready message. Wait for the tick or an early wake.
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
			}

			continue
		}

		s.deliver(msg)
	}
}

// claim atomically finds and locks the next ready message on this topic.
// Returns nil, nil when no message is available.
func (s *subscription) claim() (*queue.Message, error) {
	nowMs := time.Now().UnixMilli()

	tx, err := s.q.db.BeginTx(s.ctx, nil)
	if err != nil {
		return nil, err
	}

	defer func() { _ = tx.Rollback() }()

	// Recover any messages whose visibility timeout has expired.
	visibilityMs := nowMs - s.q.opts.VisibilityTimeout.Milliseconds()

	if _, err := tx.ExecContext(s.ctx,
		`UPDATE queue_messages
		   SET claimed_at = NULL, claimed_by = NULL
		 WHERE claimed_at IS NOT NULL
		   AND claimed_at < ?`,
		visibilityMs,
	); err != nil {
		return nil, err
	}

	// Find one ready message.
	var (
		id           int64
		topic        string
		body         []byte
		attempts     int
		enqueuedAtMs int64
		deliverAtMs  int64
		metaRaw      sql.NullString
	)

	row := tx.QueryRowContext(s.ctx,
		`SELECT id, topic, body, attempts, enqueued_at, deliver_at, metadata
		   FROM queue_messages
		  WHERE topic = ?
		    AND claimed_at IS NULL
		    AND deliver_at <= ?
		  ORDER BY id
		  LIMIT 1`,
		s.topic, nowMs,
	)

	if err := row.Scan(&id, &topic, &body, &attempts, &enqueuedAtMs, &deliverAtMs, &metaRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_ = tx.Commit()

			return nil, nil
		}

		return nil, err
	}

	// Claim it.
	if _, err := tx.ExecContext(s.ctx,
		`UPDATE queue_messages
		   SET claimed_at = ?, claimed_by = ?, attempts = attempts + 1
		 WHERE id = ?`,
		nowMs, s.q.consumerID, id,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var meta map[string]string

	if metaRaw.Valid && metaRaw.String != "" {
		if err := json.Unmarshal([]byte(metaRaw.String), &meta); err != nil {
			// Bad metadata should not tank the message.
			meta = nil
		}
	}

	return &queue.Message{
		ID:         strconv.FormatInt(id, 10),
		Topic:      topic,
		Body:       body,
		Attempts:   attempts + 1,
		EnqueuedAt: time.UnixMilli(enqueuedAtMs),
		DeliverAt:  time.UnixMilli(deliverAtMs),
		Metadata:   meta,
	}, nil
}

// deliver runs the handler with panic recovery. On success the message
// row is deleted; on error it is either requeued (with visibility
// cleared) or dropped when MaxRetries is exhausted.
func (s *subscription) deliver(msg *queue.Message) {
	err := s.safeInvoke(msg)

	if err == nil {
		// Ack: delete the row.
		_, _ = s.q.db.ExecContext(context.Background(),
			`DELETE FROM queue_messages WHERE id = ?`, msg.ID)

		return
	}

	if s.q.opts.MaxRetries > 0 && msg.Attempts >= s.q.opts.MaxRetries {
		// Drop.
		_, _ = s.q.db.ExecContext(context.Background(),
			`DELETE FROM queue_messages WHERE id = ?`, msg.ID)

		return
	}

	// Requeue: clear claim, defer by RetryDelay.
	nextMs := time.Now().UnixMilli()
	if s.q.opts.RetryDelay > 0 {
		nextMs += s.q.opts.RetryDelay.Milliseconds()
	}

	_, _ = s.q.db.ExecContext(context.Background(),
		`UPDATE queue_messages
		    SET claimed_at = NULL, claimed_by = NULL, deliver_at = ?
		  WHERE id = ?`,
		nextMs, msg.ID,
	)
}

func (s *subscription) safeInvoke(msg *queue.Message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()

	return s.handler(s.ctx, msg)
}

// compile-time proof
var _ queue.Queue = (*Queue)(nil)
