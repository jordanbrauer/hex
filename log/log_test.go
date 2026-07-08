package log_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	charm "github.com/charmbracelet/log"

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

// captureSlog returns a *slog.Logger + buffer wired to a JSON handler so
// individual tests can assert on structured output without depending
// on charmbracelet's text renderer.
func captureSlog(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	return slog.New(handler), &buf
}

func TestHandler_returnsCharmHandlerAfterInit(t *testing.T) {
	log.Init(log.WithLevel(log.DebugLevel))

	h := log.Handler()
	if h == nil {
		t.Fatalf("Handler() = nil after Init")
	}

	if _, ok := h.(*charm.Logger); !ok {
		t.Errorf("Handler() = %T, want *charm.Logger", h)
	}
}

func TestLogger_returnsSlogLoggerAfterInit(t *testing.T) {
	log.Init(log.WithLevel(log.DebugLevel))

	if log.Logger() == nil {
		t.Fatalf("Logger() = nil after Init")
	}
}

func TestWith_attachesAttrsToEveryRecord(t *testing.T) {
	// Swap slog's default for a JSON capture so we can assert on the
	// structured output. Restore after the test.
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	sl, buf := captureSlog(t)
	slog.SetDefault(sl)

	scoped := log.With(log.String("request_id", "abc"), log.Int("user", 42))
	scoped.Info("hit", log.String("route", "/x"))

	out := buf.String()
	if !strings.Contains(out, `"request_id":"abc"`) {
		t.Errorf("expected request_id in output, got: %s", out)
	}

	if !strings.Contains(out, `"user":42`) {
		t.Errorf("expected user in output, got: %s", out)
	}

	if !strings.Contains(out, `"route":"/x"`) {
		t.Errorf("expected route in output, got: %s", out)
	}
}

func TestGroup_nestsAttrsUnderKey(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	sl, buf := captureSlog(t)
	slog.SetDefault(sl)

	log.Info("paid",
		log.Group("http",
			log.String("method", "POST"),
			log.Int("status", 201)),
		log.Group("user",
			log.String("id", "u-42")),
	)

	out := buf.String()
	if !strings.Contains(out, `"http":{"method":"POST","status":201}`) {
		t.Errorf("expected nested http group, got: %s", out)
	}

	if !strings.Contains(out, `"user":{"id":"u-42"}`) {
		t.Errorf("expected nested user group, got: %s", out)
	}
}

func TestWithGroup_prefixesSubsequentAttrs(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	sl, buf := captureSlog(t)
	slog.SetDefault(sl)

	api := log.WithGroup("api")
	api.Info("call", log.String("endpoint", "/users"), log.Int("latency_ms", 12))

	out := buf.String()
	if !strings.Contains(out, `"api":{"endpoint":"/users","latency_ms":12}`) {
		t.Errorf("expected api group prefix, got: %s", out)
	}
}

func TestLogger_carriesContext(t *testing.T) {
	// Contract: Logger() returns a slog.Logger that accepts context via
	// LogAttrs so consumers can propagate trace-ids etc. through slog's
	// standard machinery. We do not decode the context here — we just
	// verify the call compiles and does not panic.
	log.Init(log.WithLevel(log.InfoLevel))

	ctx := context.Background()
	log.Logger().LogAttrs(ctx, log.InfoLevel, "ctx test",
		log.String("trace_id", "t-1"),
	)
}
