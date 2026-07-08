package ai

import (
	"context"
	"sync"
)

// ConversationStore persists multi-turn message history keyed by an
// opaque conversation ID. Callers choose the ID — a Slack thread
// timestamp, a chat session UUID, an HTTP session token — and the
// store treats it as a string.
//
// Implementations are expected to be safe for concurrent use: one
// goroutine reading history for a conversation while another appends
// a new turn must not corrupt state.
type ConversationStore interface {
	// Load returns the ordered message history for a conversation.
	// Unknown IDs yield an empty slice + nil error — a fresh
	// conversation is indistinguishable from an unseen one.
	Load(ctx context.Context, id string) ([]Message, error)

	// Save persists the messages for a conversation, replacing any
	// existing history. Callers typically load, append their new
	// turn, and save the merged slice.
	Save(ctx context.Context, id string, messages []Message) error

	// Delete removes a conversation's history. Missing IDs are not an
	// error.
	Delete(ctx context.Context, id string) error
}

// NewMemoryConversationStore returns a ConversationStore backed by an
// in-process map. Suitable for tests, single-instance apps, and any
// case where losing history on restart is acceptable. For persistent
// storage a future hex/ai/sqlite subpackage (or a custom impl backed
// by hex/db, redis, etc.) plugs in transparently.
func NewMemoryConversationStore() *MemoryConversationStore {
	return &MemoryConversationStore{
		conversations: make(map[string][]Message),
	}
}

// MemoryConversationStore is the default in-memory implementation.
type MemoryConversationStore struct {
	mu            sync.RWMutex
	conversations map[string][]Message
}

// Load returns a copy of the conversation's messages so callers can
// safely mutate the result.
func (s *MemoryConversationStore) Load(_ context.Context, id string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	src, ok := s.conversations[id]
	if !ok {
		return nil, nil
	}

	out := make([]Message, len(src))
	copy(out, src)

	return out, nil
}

// Save persists a copy of messages so later mutations by the caller
// do not leak into the store.
func (s *MemoryConversationStore) Save(_ context.Context, id string, messages []Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := make([]Message, len(messages))
	copy(stored, messages)
	s.conversations[id] = stored

	return nil
}

// Delete removes a conversation.
func (s *MemoryConversationStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.conversations, id)

	return nil
}
