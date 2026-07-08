package log_test

import (
	"testing"

	"github.com/jordanbrauer/hex/log"
)

func TestInit_defaultsToFatalLevel(t *testing.T) {
	// Sanity: force an off-default state, then re-Init and confirm it resets.
	log.SetLevel(log.DebugLevel)
	log.Init()

	if got := log.GetLevel(); got != log.FatalLevel {
		t.Errorf("GetLevel() after default Init = %v, want %v", got, log.FatalLevel)
	}
}

func TestInit_withLevelOverridesDefault(t *testing.T) {
	log.Init(log.WithLevel(log.WarnLevel))

	if got := log.GetLevel(); got != log.WarnLevel {
		t.Errorf("GetLevel() = %v, want %v", got, log.WarnLevel)
	}
}

func TestInit_isIdempotentLastCallWins(t *testing.T) {
	log.Init(log.WithLevel(log.DebugLevel))
	log.Init(log.WithLevel(log.ErrorLevel))

	if got := log.GetLevel(); got != log.ErrorLevel {
		t.Errorf("GetLevel() after second Init = %v, want %v", got, log.ErrorLevel)
	}
}

func TestSetLevel(t *testing.T) {
	log.SetLevel(log.InfoLevel)

	if got := log.GetLevel(); got != log.InfoLevel {
		t.Errorf("GetLevel() = %v, want %v", got, log.InfoLevel)
	}
}

func TestParseLevel(t *testing.T) {
	tests := map[string]log.Level{
		"debug": log.DebugLevel,
		"info":  log.InfoLevel,
		"warn":  log.WarnLevel,
		"error": log.ErrorLevel,
		"fatal": log.FatalLevel,
	}

	for input, want := range tests {
		got, err := log.ParseLevel(input)
		if err != nil {
			t.Errorf("ParseLevel(%q) error = %v", input, err)

			continue
		}

		if got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParseLevel_invalidReturnsError(t *testing.T) {
	if _, err := log.ParseLevel("chatty"); err == nil {
		t.Errorf("ParseLevel(\"chatty\") returned nil error")
	}
}

// Restore level between test runs so ordering does not leak state to
// subsequent tests or other packages.
func TestMain_resetLevel(t *testing.T) {
	t.Cleanup(func() { log.SetLevel(log.FatalLevel) })
}
