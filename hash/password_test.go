package hash_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jordanbrauer/hex/hash"
)

func TestHashPassword_roundTrip(t *testing.T) {
	pw := "correcthorsebatterystaple"

	encoded, err := hash.HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !strings.HasPrefix(encoded, "$argon2id$") {
		t.Errorf("encoded = %q, want $argon2id$ prefix", encoded)
	}

	if err := hash.VerifyPassword(pw, encoded); err != nil {
		t.Errorf("VerifyPassword(correct) = %v, want nil", err)
	}
}

func TestVerifyPassword_wrongReturnsMismatch(t *testing.T) {
	encoded, _ := hash.HashPassword("s3cret")

	err := hash.VerifyPassword("wrong", encoded)
	if !errors.Is(err, hash.ErrMismatch) {
		t.Errorf("VerifyPassword(wrong) = %v, want ErrMismatch", err)
	}
}

func TestHashPassword_producesDifferentHashesEachTime(t *testing.T) {
	// Same password + different salt (random) → different encoded string.
	a, _ := hash.HashPassword("same-password")
	b, _ := hash.HashPassword("same-password")

	if a == b {
		t.Errorf("HashPassword returned identical strings for same input; salt not random?")
	}

	// Both still verify.
	if err := hash.VerifyPassword("same-password", a); err != nil {
		t.Errorf("a: %v", err)
	}

	if err := hash.VerifyPassword("same-password", b); err != nil {
		t.Errorf("b: %v", err)
	}
}

func TestHashPassword_emptyRejected(t *testing.T) {
	if _, err := hash.HashPassword(""); err == nil {
		t.Errorf("empty password did not error")
	}
}

func TestVerifyPassword_emptyRejected(t *testing.T) {
	if err := hash.VerifyPassword("", "encoded"); err == nil {
		t.Errorf("empty password did not error")
	}

	if err := hash.VerifyPassword("pw", ""); err == nil {
		t.Errorf("empty encoded did not error")
	}
}

func TestVerifyPassword_malformedRejected(t *testing.T) {
	tests := []string{
		"not-a-hash",
		"$bcrypt$blah",
		"$argon2id$",
		"$argon2id$v=x$m=y,t=z,p=q$salt$key",
	}

	for _, e := range tests {
		if err := hash.VerifyPassword("pw", e); err == nil {
			t.Errorf("VerifyPassword(%q) = nil, want error", e)
		}
	}
}

// -- HMAC ------------------------------------------------------------------

func TestHMACSHA256_stableAndVerify(t *testing.T) {
	key := []byte("shhh")
	msg := []byte("hello")

	a := hash.HMACSHA256(key, msg)
	b := hash.HMACSHA256(key, msg)

	if a != b {
		t.Errorf("HMAC-SHA256 not deterministic: %q vs %q", a, b)
	}

	if !hash.VerifyHMACSHA256(key, msg, a) {
		t.Errorf("VerifyHMACSHA256(correct) = false")
	}
}

func TestHMACSHA256_verifyRejectsWrong(t *testing.T) {
	key := []byte("shhh")

	if hash.VerifyHMACSHA256(key, []byte("hello"), hash.HMACSHA256(key, []byte("goodbye"))) {
		t.Errorf("verify accepted MAC for wrong message")
	}

	if hash.VerifyHMACSHA256([]byte("other"), []byte("hello"), hash.HMACSHA256(key, []byte("hello"))) {
		t.Errorf("verify accepted MAC computed under different key")
	}
}

func TestHMACSHA512_roundTrip(t *testing.T) {
	key := []byte("shhh")
	msg := []byte("hello")

	m := hash.HMACSHA512(key, msg)
	if !hash.VerifyHMACSHA512(key, msg, m) {
		t.Errorf("HMAC-SHA512 verify failed")
	}
}

func TestVerifyHMAC_malformedMACReturnsFalse(t *testing.T) {
	if hash.VerifyHMACSHA256([]byte("k"), []byte("m"), "not hex") {
		t.Errorf("verify accepted malformed hex MAC")
	}
}
