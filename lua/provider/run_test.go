package provider_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/jordanbrauer/hex"
	hexlua "github.com/jordanbrauer/hex/lua"
	luaprovider "github.com/jordanbrauer/hex/lua/provider"
)

// newBootedApp constructs and boots a minimal hex.App with the Lua
// provider so the container binds *hexlua.Environment under "lua".
func newBootedApp(t *testing.T) *hex.App {
	t.Helper()

	app := hex.New()

	if err := app.Register(&luaprovider.Provider{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := app.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })

	return app
}

func TestRunCommand_inlineLua(t *testing.T) {
	app := newBootedApp(t)

	env, _ := app.Container().Make("lua")
	luaEnv := env.(*hexlua.Environment)

	var out bytes.Buffer
	luaEnv.SetStdout(&out)

	cmd := luaprovider.RunCommand(app)
	cmd.SetArgs([]string{"-c", `print("hello from lua")`})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "hello from lua" {
		t.Errorf("stdout = %q, want %q", got, "hello from lua")
	}
}

func TestRunCommand_stdinFennel(t *testing.T) {
	app := newBootedApp(t)

	env, _ := app.Container().Make("lua")
	luaEnv := env.(*hexlua.Environment)

	var out bytes.Buffer
	luaEnv.SetStdout(&out)

	cmd := luaprovider.RunCommand(app)
	cmd.SetIn(strings.NewReader(`(print (+ 1 2 3))`))
	cmd.SetArgs([]string{"-", "--lang", "fnl"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "6" {
		t.Errorf("stdout = %q, want %q", got, "6")
	}
}

func TestRunCommand_checkTealValid(t *testing.T) {
	app := newBootedApp(t)

	cmd := luaprovider.RunCommand(app)
	cmd.SetArgs([]string{"-c", "local x: number = 42", "--check", "--lang", "teal"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Errorf("execute: %v", err)
	}
}

func TestRunCommand_conflictingFlags(t *testing.T) {
	app := newBootedApp(t)

	cmd := luaprovider.RunCommand(app)
	cmd.SetArgs([]string{"foo.lua", "-c", "print(1)"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for --code + file arg, got nil")
	}
}

func TestRunCommand_noSource(t *testing.T) {
	app := newBootedApp(t)

	cmd := luaprovider.RunCommand(app)
	cmd.SetArgs([]string{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for no source, got nil")
	}
}
