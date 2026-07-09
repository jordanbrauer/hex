package fennel_test

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/lua/fennel"
)

func newState(t *testing.T) *lua.LState {
	t.Helper()

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	return L
}

func TestLoad_installsFennelGlobal(t *testing.T) {
	L := newState(t)

	if err := fennel.Load(L); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := L.GetGlobal("fennel"); got.Type() != lua.LTTable {
		t.Fatalf("fennel global type = %v, want table", got.Type())
	}
}

func TestLoad_idempotent(t *testing.T) {
	L := newState(t)

	if err := fennel.Load(L); err != nil {
		t.Fatalf("first Load: %v", err)
	}

	if err := fennel.Load(L); err != nil {
		t.Fatalf("second Load: %v", err)
	}
}

func TestCompile_basicArithmetic(t *testing.T) {
	L := newState(t)

	src := `(+ 1 2 3)`

	luaSrc, err := fennel.Compile(L, src, "<test>")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if !strings.Contains(luaSrc, "return") {
		t.Errorf("compiled Lua missing return: %s", luaSrc)
	}
}

func TestCompile_execute(t *testing.T) {
	L := newState(t)

	// A small Fennel program that does side effects + returns a value.
	src := `(local greet (fn [name] (.. "hello, " name)))
(greet "world")`

	luaSrc, err := fennel.Compile(L, src, "<test>")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if err := L.DoString(luaSrc); err != nil {
		t.Fatalf("execute: %v", err)
	}
}

func TestCompile_syntaxErrorSurfaces(t *testing.T) {
	L := newState(t)

	// Unbalanced paren \u2014 should error.
	src := `(+ 1 2`

	_, err := fennel.Compile(L, src, "<test>")
	if err == nil {
		t.Fatalf("expected compile error for unbalanced parens")
	}
}

func TestIsFennelFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"foo.fnl", true},
		{"foo.FNL", true},
		{"path/to/foo.fnl", true},
		{"foo.lua", false},
		{"foo.tl", false},
		{"foo", false},
	}

	for _, tc := range cases {
		if got := fennel.IsFennelFile(tc.path); got != tc.want {
			t.Errorf("IsFennelFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestSession_reusesCompilerAcrossCalls(t *testing.T) {
	L := newState(t)

	sess, err := fennel.NewSession(L)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	// Two compiles against the same session should both succeed
	// without re-loading fennel.
	if _, err := sess.Compile(`(+ 1 1)`, "<a>"); err != nil {
		t.Fatalf("first compile: %v", err)
	}

	if _, err := sess.Compile(`(+ 2 2)`, "<b>"); err != nil {
		t.Fatalf("second compile: %v", err)
	}

	// Close is a no-op but should not error.
	if err := sess.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
