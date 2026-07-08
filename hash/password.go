// Package hash provides password hashing (argon2id) and HMAC signature
// helpers.
//
// Passwords: use HashPassword to produce a self-describing hash string
// (parameters + salt embedded), and VerifyPassword to check. The string
// format matches the standard PHC (argon2id) encoding used by every
// modern library, so it is portable across languages.
//
// HMAC: HMACSHA256 and HMACSHA512 return hex-encoded MACs. VerifyHMAC
// uses constant-time comparison.
package hash

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// -- password (argon2id PHC) ----------------------------------------------

// argon2Params are the tuning knobs. Values matched to the OWASP 2024
// recommendation (memory=19MiB, time=2, parallelism=1).
type argon2Params struct {
	memory      uint32 // KiB
	time        uint32
	parallelism uint8
	saltLen     uint32
	keyLen      uint32
}

// DefaultArgon2Params are the OWASP-recommended defaults. Exposed so
// consumers running on slow hardware can dial down; production should
// dial up when possible.
var DefaultArgon2Params = argon2Params{
	memory:      19 * 1024,
	time:        2,
	parallelism: 1,
	saltLen:     16,
	keyLen:      32,
}

// HashPassword returns the PHC-encoded argon2id hash of password.
func HashPassword(password string) (string, error) {
	return HashPasswordWith(password, DefaultArgon2Params)
}

// HashPasswordWith is HashPassword with explicit params.
func HashPasswordWith(password string, p argon2Params) (string, error) {
	if password == "" {
		return "", errors.New("hash: password is empty")
	}

	salt := make([]byte, p.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("hash: read salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.parallelism, p.keyLen)

	// PHC format: $argon2id$v=<v>$m=<mem>,t=<time>,p=<parallel>$<salt>$<key>
	b64 := base64.RawStdEncoding.EncodeToString

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.time, p.parallelism,
		b64(salt), b64(key),
	), nil
}

// VerifyPassword returns nil if password hashes to encoded (which must be
// a PHC-format string produced by HashPassword or any argon2id-compatible
// tool). Errors on malformed input; ErrMismatch on wrong password.
func VerifyPassword(password, encoded string) error {
	if password == "" {
		return errors.New("hash: password is empty")
	}

	if encoded == "" {
		return errors.New("hash: encoded hash is empty")
	}

	parts := strings.Split(encoded, "$")
	// PHC: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<key>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return errors.New("hash: not an argon2id PHC string")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return fmt.Errorf("hash: parse version: %w", err)
	}

	var mem, t uint32
	var par uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &par); err != nil {
		return fmt.Errorf("hash: parse params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return fmt.Errorf("hash: decode salt: %w", err)
	}

	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return fmt.Errorf("hash: decode key: %w", err)
	}

	got := argon2.IDKey([]byte(password), salt, t, mem, par, uint32(len(want)))

	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}

	return nil
}

// ErrMismatch is returned by VerifyPassword when the password does not
// match the encoded hash. Distinct from format-parsing errors so callers
// can react (e.g. increment failure counters) only for real mismatches.
var ErrMismatch = errors.New("hash: password mismatch")

// -- HMAC -----------------------------------------------------------------

// HMACSHA256 returns hex(HMAC-SHA256(key, msg)).
func HMACSHA256(key, msg []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(msg)

	return hex.EncodeToString(h.Sum(nil))
}

// HMACSHA512 returns hex(HMAC-SHA512(key, msg)).
func HMACSHA512(key, msg []byte) string {
	h := hmac.New(sha512.New, key)
	h.Write(msg)

	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHMACSHA256 constant-time compares mac (hex string) against the
// HMAC of msg under key.
func VerifyHMACSHA256(key, msg []byte, mac string) bool {
	return verifyHex(HMACSHA256(key, msg), mac)
}

// VerifyHMACSHA512 constant-time compares mac (hex string) against the
// HMAC of msg under key.
func VerifyHMACSHA512(key, msg []byte, mac string) bool {
	return verifyHex(HMACSHA512(key, msg), mac)
}

func verifyHex(want, got string) bool {
	// Decode-then-compare so length differences do not leak via string
	// comparison; ConstantTimeCompare short-circuits on length mismatch
	// but the exit is safe (an attacker learns only that lengths differ).
	wb, err1 := hex.DecodeString(want)
	gb, err2 := hex.DecodeString(got)

	if err1 != nil || err2 != nil {
		return false
	}

	return subtle.ConstantTimeCompare(wb, gb) == 1
}
