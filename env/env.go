// Package env names the runtime environment a hex application is
// running in and provides detection helpers.
//
// hex ships three canonical environments:
//
//	Development  running the app locally while working on it (go run .)
//	Test         go test ..., CI/CD
//	Production   released binaries in staging, production, anywhere
//	             the app is deployed for real work
//
// The set is deliberately small. If you need to distinguish staging
// from production, that is a deployment concern — read a separate
// config key like `deploy.tier` instead of adding an environment.
//
// Detection order (see Detect):
//
//  1. HEX_ENV / APP_ENV environment variables
//  2. testing.Testing() — Test when running under `go test`
//  3. Development as the default
//
// Explicit overrides via hex.WithEnvironment take precedence over all
// of the above.
package env

import (
	"os"
	"strings"
	"testing"
)

// Environment is a strict enum: only Development, Test, and
// Production are valid values. Parse accepts a small set of aliases;
// otherwise it returns an error so misspelled overrides fail loudly.
type Environment string

const (
	// Development is the local-development environment.
	Development Environment = "development"

	// Test is the test / CI environment.
	Test Environment = "test"

	// Production is the deployed / released environment.
	Production Environment = "production"
)

// All returns every recognised environment in a stable order. Useful
// for CLI help text and validation.
func All() []Environment {
	return []Environment{Development, Test, Production}
}

// Parse accepts case-insensitive names and common aliases. Returns an
// error for unknown strings so config validation catches typos rather
// than silently defaulting.
func Parse(s string) (Environment, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "development", "dev", "local":
		return Development, nil
	case "test", "testing", "ci":
		return Test, nil
	case "production", "prod", "live":
		return Production, nil
	default:
		return "", &UnknownError{Value: s}
	}
}

// UnknownError is returned by Parse when the input does not name a
// recognised environment.
type UnknownError struct {
	Value string
}

func (e *UnknownError) Error() string {
	return "env: unknown environment " + strconvQuote(e.Value) + " (want development, test, or production)"
}

// Detect returns the environment inferred from the process, using the
// order documented on the package.
func Detect() Environment {
	for _, key := range []string{"HEX_ENV", "APP_ENV"} {
		if v := os.Getenv(key); v != "" {
			if e, err := Parse(v); err == nil {
				return e
			}
		}
	}

	if testing.Testing() {
		return Test
	}

	return Development
}

// IsDev reports Development.
func (e Environment) IsDev() bool { return e == Development }

// IsTest reports Test.
func (e Environment) IsTest() bool { return e == Test }

// IsProduction reports Production.
func (e Environment) IsProduction() bool { return e == Production }

// String returns the environment's canonical name.
func (e Environment) String() string { return string(e) }

// strconvQuote wraps a string in double quotes without pulling strconv
// into an otherwise dependency-free package.
func strconvQuote(s string) string {
	return "\"" + s + "\""
}
