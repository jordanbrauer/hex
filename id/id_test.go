package id_test

import (
	"testing"

	"github.com/jordanbrauer/hex/id"
)

func TestUUIDv4_uniqueAndParsable(t *testing.T) {
	a, b := id.UUIDv4(), id.UUIDv4()
	if a == b {
		t.Errorf("UUIDv4 collision")
	}

	if id.Parse(a) != id.KindUUID {
		t.Errorf("Parse(%q) != KindUUID", a)
	}
}

func TestUUIDv7_uniqueAndParsable(t *testing.T) {
	a, b := id.UUIDv7(), id.UUIDv7()
	if a == b {
		t.Errorf("UUIDv7 collision")
	}

	if id.Parse(a) != id.KindUUID {
		t.Errorf("Parse(%q) != KindUUID", a)
	}
}

func TestULID_uniqueAndParsable(t *testing.T) {
	a, b := id.ULID(), id.ULID()
	if a == b {
		t.Errorf("ULID collision")
	}

	if id.Parse(a) != id.KindULID {
		t.Errorf("Parse(%q) = %v, want KindULID", a, id.Parse(a))
	}
}

func TestKSUID_uniqueAndParsable(t *testing.T) {
	a, b := id.KSUID(), id.KSUID()
	if a == b {
		t.Errorf("KSUID collision")
	}

	if id.Parse(a) != id.KindKSUID {
		t.Errorf("Parse(%q) = %v, want KindKSUID", a, id.Parse(a))
	}
}

func TestParse_unknownReturnsUnknown(t *testing.T) {
	tests := []string{
		"",
		"not an id",
		"12345",
	}

	for _, tt := range tests {
		if got := id.Parse(tt); got != id.KindUnknown {
			t.Errorf("Parse(%q) = %v, want KindUnknown", tt, got)
		}
	}
}

func TestKind_string(t *testing.T) {
	tests := map[id.Kind]string{
		id.KindUnknown: "unknown",
		id.KindUUID:    "uuid",
		id.KindULID:    "ulid",
		id.KindKSUID:   "ksuid",
	}

	for k, want := range tests {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}
