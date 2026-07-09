package teal_test

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/lua/teal"
)

func newState(t *testing.T) *lua.LState {
	t.Helper()

	L := lua.NewState()
	t.Cleanup(func() { L.Close() })

	if err := teal.Load(L); err != nil {
		t.Fatalf("teal.Load: %v", err)
	}

	return L
}

func TestLoad_installsTlGlobal(t *testing.T) {
	L := newState(t)

	if err := L.DoString(`
		if type(tl) ~= "table" then
			error("tl global not installed: " .. type(tl))
		end
		if type(tl.process_string) ~= "function" then
			error("tl.process_string missing")
		end
	`); err != nil {
		t.Fatalf("global check: %v", err)
	}
}

func TestCompile_simpleFunction(t *testing.T) {
	L := newState(t)

	src := `local function add(a: number, b: number): number
    return a + b
end
return add(3, 4)
`

	out, err := teal.Compile(L, src, "add.tl")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	if !strings.Contains(out, "return a + b") {
		t.Errorf("compiled output missing function body:\n%s", out)
	}

	// The compiled output should be executable plain Lua.
	if err := L.DoString(out); err != nil {
		t.Fatalf("exec compiled output: %v", err)
	}
}

func TestCompile_reportsSyntaxErrors(t *testing.T) {
	L := newState(t)

	// Missing 'end'.
	src := `local function broken(x: number): number
    return x + 1
`

	_, err := teal.Compile(L, src, "broken.tl")
	if err == nil {
		t.Fatalf("Compile succeeded on broken source; want error")
	}
}

func TestCompile_reportsTypeErrors(t *testing.T) {
	L := newState(t)

	// Assign string to number.
	src := `local x: number = "not a number"
return x
`

	_, err := teal.Compile(L, src, "typo.tl")
	if err == nil {
		t.Fatalf("Compile succeeded on type mismatch; want error")
	}

	if !strings.Contains(err.Error(), "type error") && !strings.Contains(err.Error(), "cannot") {
		t.Logf("compile error was: %v", err)
	}
}

func TestCheck_returnsNilOnClean(t *testing.T) {
	L := newState(t)

	src := `local x: number = 42
return x
`

	if err := teal.Check(L, src, "clean.tl"); err != nil {
		t.Fatalf("Check on clean source returned error: %v", err)
	}
}

func TestCheck_returnsErrorsOnBadType(t *testing.T) {
	L := newState(t)

	src := `local x: number = "bad"
return x
`

	if err := teal.Check(L, src, "bad.tl"); err == nil {
		t.Fatalf("Check succeeded on type-error source; want error")
	}
}
