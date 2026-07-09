package lua_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	loglua "github.com/jordanbrauer/hex/log/lua"
	hexlua "github.com/jordanbrauer/hex/lua"
)

// captureSlog swaps slog.Default() for a text handler writing into
// buf. Returns a restore func the test defers.
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer

	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	t.Cleanup(func() { slog.SetDefault(previous) })

	return &buf
}

func newEnv(t *testing.T) *hexlua.Environment {
	t.Helper()

	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	bindings := &loglua.Bindings{}
	env.PreloadModule("log", bindings.Loader)

	return env
}

func TestLog_infoWritesToSlog(t *testing.T) {
	buf := captureSlog(t)
	env := newEnv(t)

	err := env.ExecString(`
		local log = require("log")
		log.info("hello from lua", { user = "alice", count = 42 })
	`, "log_info.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "hello from lua") {
		t.Errorf("msg missing: %s", out)
	}

	if !strings.Contains(out, "user=alice") {
		t.Errorf("user attr missing: %s", out)
	}

	if !strings.Contains(out, "count=42") {
		t.Errorf("count attr missing: %s", out)
	}
}

func TestLog_allLevels(t *testing.T) {
	buf := captureSlog(t)
	env := newEnv(t)

	err := env.ExecString(`
		local log = require("log")
		log.debug("d")
		log.info("i")
		log.warn("w")
		log.error("e")
	`, "log_levels.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"level=DEBUG", "level=INFO", "level=WARN", "level=ERROR"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestLog_messageOnlyIsFine(t *testing.T) {
	buf := captureSlog(t)
	env := newEnv(t)

	err := env.ExecString(`
		local log = require("log")
		log.info("no attrs")
	`, "log_no_attrs.lua")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	if !strings.Contains(buf.String(), "no attrs") {
		t.Errorf("msg missing: %s", buf.String())
	}
}
