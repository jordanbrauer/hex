package env_test

import (
	"errors"
	"testing"

	"github.com/jordanbrauer/hex/env"
)

func TestDetect_underTestingReturnsTest(t *testing.T) {
	// This test runs under `go test`, so testing.Testing() is true
	// and — barring an env-var override — Detect should return Test.
	t.Setenv("HEX_ENV", "")
	t.Setenv("APP_ENV", "")

	if got := env.Detect(); got != env.Test {
		t.Errorf("Detect() = %v, want Test", got)
	}
}

func TestDetect_hexEnvOverrides(t *testing.T) {
	t.Setenv("HEX_ENV", "production")
	t.Setenv("APP_ENV", "")

	if got := env.Detect(); got != env.Production {
		t.Errorf("Detect() = %v, want Production", got)
	}
}

func TestDetect_appEnvOverrides(t *testing.T) {
	t.Setenv("HEX_ENV", "")
	t.Setenv("APP_ENV", "development")

	if got := env.Detect(); got != env.Development {
		t.Errorf("Detect() = %v, want Development", got)
	}
}

func TestDetect_invalidHexEnvFallsThroughToTestUnderTesting(t *testing.T) {
	// Invalid env-var strings should not silently pick something —
	// Detect falls through to testing.Testing() which returns Test.
	t.Setenv("HEX_ENV", "gibberish")
	t.Setenv("APP_ENV", "")

	if got := env.Detect(); got != env.Test {
		t.Errorf("Detect() = %v, want Test (fallthrough)", got)
	}
}

func TestParse_recognisedNames(t *testing.T) {
	cases := map[string]env.Environment{
		"":            env.Development,
		"development": env.Development,
		"Development": env.Development,
		"DEV":         env.Development,
		"local":       env.Development,
		"test":        env.Test,
		"Testing":     env.Test,
		"ci":          env.Test,
		"production":  env.Production,
		"prod":        env.Production,
		"LIVE":        env.Production,
	}

	for input, want := range cases {
		got, err := env.Parse(input)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", input, err)

			continue
		}

		if got != want {
			t.Errorf("Parse(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParse_unknownReturnsError(t *testing.T) {
	_, err := env.Parse("staging")

	var uk *env.UnknownError
	if !errors.As(err, &uk) {
		t.Fatalf("error = %T, want *UnknownError", err)
	}

	if uk.Value != "staging" {
		t.Errorf("UnknownError.Value = %q, want %q", uk.Value, "staging")
	}
}

func TestPredicates(t *testing.T) {
	if !env.Development.IsDev() {
		t.Error("Development.IsDev() false")
	}

	if !env.Test.IsTest() {
		t.Error("Test.IsTest() false")
	}

	if !env.Production.IsProduction() {
		t.Error("Production.IsProduction() false")
	}

	if env.Test.IsDev() || env.Test.IsProduction() {
		t.Error("Test predicates leaked")
	}
}
