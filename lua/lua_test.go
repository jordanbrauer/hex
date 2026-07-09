package lua_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gopherlua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/lua"
)

func TestNew_isUsable(t *testing.T) {
	env := lua.New()
	defer env.Close()

	if env.L == nil {
		t.Errorf("env.L is nil after New")
	}
}

func TestClose_isIdempotent(t *testing.T) {
	env := lua.New()

	if err := env.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	if err := env.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestExecString_runsScript(t *testing.T) {
	env := lua.New()
	defer env.Close()

	if err := env.ExecString(`x = 1 + 2`, "arith"); err != nil {
		t.Fatalf("ExecString: %v", err)
	}

	got := env.GetGlobal("x")
	if n, ok := got.(gopherlua.LNumber); !ok || float64(n) != 3 {
		t.Errorf("x = %v, want 3", got)
	}
}

func TestExecString_syntaxErrorReturned(t *testing.T) {
	env := lua.New()
	defer env.Close()

	err := env.ExecString(`this is not valid lua =`, "bad")
	if err == nil {
		t.Errorf("bad syntax returned nil error")
	}
}

func TestExecString_runtimeErrorReturned(t *testing.T) {
	env := lua.New()
	defer env.Close()

	err := env.ExecString(`error("boom")`, "runtime")
	if err == nil {
		t.Errorf("Lua error() returned nil error")

		return
	}

	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want to contain 'boom'", err)
	}
}

func TestExecFile_readsAndRuns(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "s.lua")

	if err := os.WriteFile(fp, []byte(`y = "hello"`), 0o644); err != nil {
		t.Fatal(err)
	}

	env := lua.New()
	defer env.Close()

	if err := env.ExecFile(fp); err != nil {
		t.Fatalf("ExecFile: %v", err)
	}

	got := env.GetGlobal("y")
	if s, ok := got.(gopherlua.LString); !ok || string(s) != "hello" {
		t.Errorf("y = %v, want hello", got)
	}
}

func TestExecFile_missingReturnsError(t *testing.T) {
	env := lua.New()
	defer env.Close()

	if err := env.ExecFile("/nonexistent/foo.lua"); err == nil {
		t.Errorf("ExecFile on missing path returned nil error")
	}
}

func TestCompile_reusableAcrossExecs(t *testing.T) {
	script, err := lua.Compile(`counter = (counter or 0) + 1`, "counter")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if script.Name() != "counter" {
		t.Errorf("Name = %q, want counter", script.Name())
	}

	env := lua.New()
	defer env.Close()

	for i := 1; i <= 3; i++ {
		if err := env.Exec(script); err != nil {
			t.Fatalf("Exec %d: %v", i, err)
		}
	}

	got := env.GetGlobal("counter")
	if n, ok := got.(gopherlua.LNumber); !ok || int(n) != 3 {
		t.Errorf("counter = %v, want 3", got)
	}
}

func TestCompile_sharedAcrossEnvironments(t *testing.T) {
	script, err := lua.Compile(`shared_flag = true`, "shared")
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		env := lua.New()

		if err := env.Exec(script); err != nil {
			env.Close()
			t.Fatalf("env %d Exec: %v", i, err)
		}

		got := env.GetGlobal("shared_flag")
		if b, ok := got.(gopherlua.LBool); !ok || !bool(b) {
			t.Errorf("env %d shared_flag = %v, want true", i, got)
		}

		env.Close()
	}
}

func TestCompile_badSourceReturnsError(t *testing.T) {
	if _, err := lua.Compile(`if end`, "bad"); err == nil {
		t.Errorf("Compile bad source returned nil error")
	}
}

func TestSetGlobal_primitives(t *testing.T) {
	env := lua.New()
	defer env.Close()

	tests := map[string]any{
		"s":  "hello",
		"b":  true,
		"i":  int(42),
		"i6": int64(42),
		"u6": uint64(42),
		"f":  3.14,
		"n":  nil,
	}

	for name, val := range tests {
		if err := env.SetGlobal(name, val); err != nil {
			t.Errorf("SetGlobal %s: %v", name, err)
		}
	}

	// Sanity: read a couple back through Lua.
	if err := env.ExecString(`assert(s == "hello" and b == true and i == 42 and f > 3.13, "globals mismatch")`, "assert"); err != nil {
		t.Errorf("assert failed: %v", err)
	}
}

func TestSetGlobal_unsupportedTypeErrors(t *testing.T) {
	env := lua.New()
	defer env.Close()

	type unsupported struct{ X int }

	if err := env.SetGlobal("bad", unsupported{}); err == nil {
		t.Errorf("SetGlobal with struct returned nil error")
	}
}

func TestSetGlobal_luaValuePassesThrough(t *testing.T) {
	env := lua.New()
	defer env.Close()

	tbl := env.L.NewTable()
	tbl.RawSetString("k", gopherlua.LString("v"))

	if err := env.SetGlobal("t", tbl); err != nil {
		t.Fatalf("SetGlobal table: %v", err)
	}

	if err := env.ExecString(`assert(t.k == "v", "table")`, "assert"); err != nil {
		t.Errorf("assert: %v", err)
	}
}

func TestSetStdout_redirectsPrint(t *testing.T) {
	env := lua.New()
	t.Cleanup(func() { _ = env.Close() })

	var buf strings.Builder
	env.SetStdout(&buf)

	if err := env.ExecString(`print("hello", 42, true)`, "print.lua"); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "hello") {
		t.Errorf("missing 'hello': %q", got)
	}

	if !strings.Contains(got, "42") {
		t.Errorf("missing '42': %q", got)
	}

	if !strings.Contains(got, "true") {
		t.Errorf("missing 'true': %q", got)
	}

	if !strings.HasSuffix(got, "\n") {
		t.Errorf("missing trailing newline: %q", got)
	}
}

func TestSetStdout_nilRestoresDefault(t *testing.T) {
	env := lua.New()
	t.Cleanup(func() { _ = env.Close() })

	var buf strings.Builder
	env.SetStdout(&buf)
	env.SetStdout(nil)

	// Just verify it didn't panic and the writer is non-nil.
	if env.Stdout() == nil {
		t.Errorf("nil SetStdout should restore os.Stdout, got nil")
	}
}

func TestPreloadModule_isRequirable(t *testing.T) {
	env := lua.New()
	defer env.Close()

	loader := func(L *gopherlua.LState) int {
		tbl := L.NewTable()
		L.SetField(tbl, "greet", L.NewFunction(func(L *gopherlua.LState) int {
			L.Push(gopherlua.LString("hello, " + L.CheckString(1)))

			return 1
		}))
		L.Push(tbl)

		return 1
	}

	env.PreloadModule("hex_test", loader)

	err := env.ExecString(
		`local m = require("hex_test") ; result = m.greet("world")`,
		"require",
	)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	got := env.GetGlobal("result")
	if s, ok := got.(gopherlua.LString); !ok || string(s) != "hello, world" {
		t.Errorf("result = %v, want 'hello, world'", got)
	}
}

func TestWithoutStandardLibraries(t *testing.T) {
	env := lua.New(lua.WithoutStandardLibraries())
	defer env.Close()

	// `print` is part of the base library; without stdlib it is nil.
	err := env.ExecString(`print("x")`, "print")
	if err == nil {
		t.Errorf("print worked without stdlib")
	}
}

func TestWithPackagePath_makesRequireFindFile(t *testing.T) {
	dir := t.TempDir()

	mod := filepath.Join(dir, "greeter.lua")

	src := `local M = {}
M.hello = function(name) return "hi, " .. name end
return M
`

	if err := os.WriteFile(mod, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	env := lua.New(lua.WithPackagePath(dir))
	defer env.Close()

	err := env.ExecString(
		`local g = require("greeter") ; result = g.hello("world")`,
		"require-file",
	)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	got := env.GetGlobal("result")
	if s, ok := got.(gopherlua.LString); !ok || string(s) != "hi, world" {
		t.Errorf("result = %v, want 'hi, world'", got)
	}
}

func TestExec_nilScript(t *testing.T) {
	env := lua.New()
	defer env.Close()

	if err := env.Exec(nil); err == nil {
		t.Errorf("Exec(nil) returned nil error")
	}
}

// -- Teal (.tl) integration -----------------------------------------------

func TestExecFile_transparentlyHandlesTeal(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "greet.tl")

	src := `global function greet(name: string): string
    return "hello, " .. name
end
global result: string = greet("teal")
`
	if err := os.WriteFile(fp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	env := lua.New()
	defer env.Close()

	if err := env.ExecFile(fp); err != nil {
		t.Fatalf("ExecFile .tl: %v", err)
	}

	got := env.GetGlobal("result")
	if s, ok := got.(gopherlua.LString); !ok || string(s) != "hello, teal" {
		t.Errorf("result = %v, want 'hello, teal'", got)
	}
}

func TestCheckFile_passesForCleanTeal(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "clean.tl")

	if err := os.WriteFile(fp, []byte(`local x: number = 42
return x
`), 0o644); err != nil {
		t.Fatal(err)
	}

	env := lua.New()
	defer env.Close()

	if err := env.CheckFile(fp); err != nil {
		t.Errorf("CheckFile clean: %v", err)
	}
}

func TestCheckFile_failsForTealTypeError(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "bad.tl")

	if err := os.WriteFile(fp, []byte(`local x: number = "not a number"
return x
`), 0o644); err != nil {
		t.Fatal(err)
	}

	env := lua.New()
	defer env.Close()

	if err := env.CheckFile(fp); err == nil {
		t.Errorf("CheckFile on bad .tl returned nil; want error")
	}
}

func TestCheckFile_luaFileParseCheck(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "clean.lua")

	if err := os.WriteFile(fp, []byte(`return 1 + 2`), 0o644); err != nil {
		t.Fatal(err)
	}

	env := lua.New()
	defer env.Close()

	if err := env.CheckFile(fp); err != nil {
		t.Errorf("CheckFile .lua: %v", err)
	}
}
