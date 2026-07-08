// Package id generates identifiers for hex applications.
//
// One consistent surface for the three ID formats we reach for most:
//
//   - UUID v4  — random 128-bit ID, no time component
//   - UUID v7  — time-sortable 128-bit ID (RFC 9562)
//   - ULID     — 128-bit time-sortable, Crockford base32
//   - KSUID    — 27-char time-sortable
//
// Prefer UUID v7 or ULID for new tables — they are sortable by insertion
// time, which speeds up index scans without giving up global uniqueness.
package id

import (
	"crypto/rand"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/segmentio/ksuid"
)

// UUIDv4 returns a random UUID v4 as a string.
func UUIDv4() string { return uuid.NewString() }

// UUIDv7 returns a time-sortable UUID v7 (RFC 9562).
func UUIDv7() string {
	u, err := uuid.NewV7()
	if err != nil {
		// Fallback to v4 rather than panic; NewV7 only errors on
		// crypto/rand failure, which is already unrecoverable.
		return uuid.NewString()
	}

	return u.String()
}

// ULID returns a Crockford base32-encoded ULID.
func ULID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}

// KSUID returns a 27-character KSUID.
func KSUID() string { return ksuid.New().String() }

// Parse attempts to identify and parse an ID string, returning the
// detected kind. Useful for logging or defensive validation. Returns
// KindUnknown when no format matches.
func Parse(s string) Kind {
	switch len(s) {
	case 36:
		if _, err := uuid.Parse(s); err == nil {
			return KindUUID
		}
	case 26:
		if _, err := ulid.Parse(s); err == nil {
			return KindULID
		}
	case 27:
		if _, err := ksuid.Parse(s); err == nil {
			return KindKSUID
		}
	}

	return KindUnknown
}

// Kind names an identifier scheme.
type Kind int

const (
	// KindUnknown means Parse could not identify the format.
	KindUnknown Kind = iota
	// KindUUID covers UUID v4 and v7 — same string shape.
	KindUUID
	// KindULID is a Crockford base32-encoded 26-char ID.
	KindULID
	// KindKSUID is a 27-char segment.io ID.
	KindKSUID
)

// String returns the human name for k.
func (k Kind) String() string {
	switch k {
	case KindUUID:
		return "uuid"
	case KindULID:
		return "ulid"
	case KindKSUID:
		return "ksuid"
	default:
		return "unknown"
	}
}
