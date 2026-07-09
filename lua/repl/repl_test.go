package repl

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	hexlua "github.com/jordanbrauer/hex/lua"
)

// withConfigDir points os.UserConfigDir at a temp dir for the
// duration of the test, so history persistence tests don't touch the
// user's real config directory.
func withConfigDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// os.UserConfigDir reads XDG_CONFIG_HOME on Linux and
	// HOME/Library/Application Support on macOS. Set both so tests
	// work regardless of platform.
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	return dir
}

func TestSaveLoadHistory_roundtrip(t *testing.T) {
	withConfigDir(t)

	want := []string{"print(1)", "global x = 2", "db.query('...')"}

	if err := saveHistory("myapp", want); err != nil {
		t.Fatalf("saveHistory: %v", err)
	}

	got := loadHistory("myapp")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch:\n got: %v\nwant: %v", got, want)
	}
}

func TestLoadHistory_missingReturnsNil(t *testing.T) {
	withConfigDir(t)

	if got := loadHistory("never-existed"); got != nil {
		t.Errorf("missing file should load as nil, got %v", got)
	}
}

func TestSaveHistory_atomicRenameLeavesNoTemp(t *testing.T) {
	dir := withConfigDir(t)

	if err := saveHistory("myapp", []string{"one", "two"}); err != nil {
		t.Fatalf("saveHistory: %v", err)
	}

	tmp := filepath.Join(dir, "myapp", "repl-history.tmp")
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("temp file should have been renamed, still present at %s", tmp)
	}
}

func TestSaveHistory_createsParentDir(t *testing.T) {
	withConfigDir(t)

	if err := saveHistory("deeply/nested/appname", []string{"x"}); err != nil {
		t.Fatalf("saveHistory should create parent dirs: %v", err)
	}

	if got := loadHistory("deeply/nested/appname"); len(got) != 1 || got[0] != "x" {
		t.Errorf("got %v, want [x]", got)
	}
}

func TestSaveHistory_emptyAppNameErrors(t *testing.T) {
	withConfigDir(t)

	if err := saveHistory("", []string{"x"}); err == nil {
		t.Errorf("saveHistory with empty appName should error")
	}
}

func TestIsIncompleteError_gopherLua(t *testing.T) {
	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	cases := []struct {
		name       string
		src        string
		incomplete bool
	}{
		{"unclosed function", "function foo()", true},
		{"unclosed table", "local x = {1,", true},
		{"unclosed if", "if true then", true},
		{"unclosed call", "foo(", true},
		{"complete function", "function foo() end", false},
		{"real syntax error", "function foo() end end", false},
		{"nil is not incomplete", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.src == "" {
				if isIncompleteError(nil, hexlua.Lua) {
					t.Errorf("nil err reported incomplete")
				}

				return
			}

			_, err := env.L.LoadString(tc.src)
			if err == nil {
				if tc.incomplete {
					t.Fatalf("expected parse to fail for %q", tc.src)
				}

				return
			}

			got := isIncompleteError(err, hexlua.Lua)
			if got != tc.incomplete {
				t.Errorf("isIncompleteError(%q) = %v, want %v; err=%q", tc.src, got, tc.incomplete, err.Error())
			}
		})
	}
}

// run is a tiny helper that wires stdin/stdout/stderr buffers and
// runs the REPL in the requested mode against a bare environment.
func run(t *testing.T, in string, mode Mode) (out, errOut string) {
	t.Helper()

	var outBuf, errBuf bytes.Buffer

	err := Run(Options{
		Mode:    mode,
		In:      strings.NewReader(in),
		Out:     &outBuf,
		ErrOut:  &errBuf,
		AppName: "test",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	return outBuf.String(), errBuf.String()
}

func TestRun_expressionsPrintValues(t *testing.T) {
	out, _ := run(t, "1 + 2\n\"hello\"\nexit\n", ModeLua)

	if !strings.Contains(out, "3") {
		t.Errorf("expected 3 in output, got:\n%s", out)
	}

	if !strings.Contains(out, "hello") {
		t.Errorf("expected hello in output, got:\n%s", out)
	}
}

func TestRun_statementsPersistGlobalsInLua(t *testing.T) {
	// Lua REPL: implicit globals persist across chunks.
	out, _ := run(t, "counter = 41\ncounter + 1\nexit\n", ModeLua)

	if !strings.Contains(out, "42") {
		t.Errorf("expected 42, got:\n%s", out)
	}
}

func TestRun_tealSessionPersistsGlobals(t *testing.T) {
	// Teal REPL: `global x: T = v` persists both type + value.
	out, errOut := run(t, "global bar: number = 12\nbar * bar\nexit\n", ModeTeal)

	if !strings.Contains(out, "144") {
		t.Errorf("expected 144, got:\n%s", out)
	}

	if errOut != "" {
		t.Errorf("stderr should be empty on happy path, got:\n%s", errOut)
	}
}

func TestRun_tealImplicitGlobalHint(t *testing.T) {
	// Teal REPL: `bar = 12` without prior `global` errors + hints.
	_, errOut := run(t, "bar = 12\nexit\n", ModeTeal)

	if !strings.Contains(errOut, "unknown variable") {
		t.Errorf("expected 'unknown variable' error, got:\n%s", errOut)
	}

	if !strings.Contains(errOut, "global") {
		t.Errorf("expected `global` hint, got:\n%s", errOut)
	}
}

func TestRun_errorDoesNotAbort(t *testing.T) {
	out, errOut := run(t, "error(\"boom\")\n\"still alive\"\nexit\n", ModeLua)

	if !strings.Contains(errOut, "boom") {
		t.Errorf("expected 'boom' on stderr, got:\n%s", errOut)
	}

	if strings.Contains(errOut, "stack traceback") {
		t.Errorf("stderr should not include stack traceback:\n%s", errOut)
	}

	if !strings.Contains(out, "still alive") {
		t.Errorf("REPL didn't continue after error:\n%s", out)
	}
}

func TestRun_exitDirectives(t *testing.T) {
	for _, kw := range []string{"exit", "quit", ".exit", ".quit"} {
		t.Run(kw, func(t *testing.T) {
			out, _ := run(t, "1\n"+kw+"\n2\n", ModeLua)

			if !strings.Contains(out, "1") {
				t.Errorf("REPL exited before evaluating first line")
			}

			if strings.Contains(out, "2") {
				t.Errorf("REPL evaluated line after %q; should have exited", kw)
			}
		})
	}
}

func TestRun_eofExits(t *testing.T) {
	out, _ := run(t, "42\n", ModeLua)

	if !strings.Contains(out, "42") {
		t.Errorf("expected 42 in output before EOF:\n%s", out)
	}
}

func TestRun_appNameInPrompt(t *testing.T) {
	var outBuf, errBuf bytes.Buffer

	err := Run(Options{
		Mode:    ModeLua,
		In:      strings.NewReader("exit\n"),
		Out:     &outBuf,
		ErrOut:  &errBuf,
		AppName: "myapp",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(outBuf.String(), "myapp") {
		t.Errorf("prompt did not include AppName, got:\n%s", outBuf.String())
	}
}

func TestRun_bannerLineIsPrinted(t *testing.T) {
	var outBuf, errBuf bytes.Buffer

	err := Run(Options{
		Mode:    ModeLua,
		In:      strings.NewReader("exit\n"),
		Out:     &outBuf,
		ErrOut:  &errBuf,
		AppName: "myapp",
		Banner:  "⚠  connected to PRODUCTION",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(outBuf.String(), "PRODUCTION") {
		t.Errorf("banner missing, got:\n%s", outBuf.String())
	}
}

func TestRun_reusesCallerEnv(t *testing.T) {
	// Caller creates and owns the env: registered modules should be
	// available in the REPL, and the env stays open after Run
	// returns.
	env := hexlua.New()
	defer env.Close()

	// Register a trivial global to prove the caller-scoped env is
	// what the REPL sees.
	if err := env.L.DoString(`hello_from_caller = "yes"`); err != nil {
		t.Fatalf("DoString: %v", err)
	}

	var outBuf, errBuf bytes.Buffer

	err := Run(Options{
		Mode:    ModeLua,
		In:      strings.NewReader("hello_from_caller\nexit\n"),
		Out:     &outBuf,
		ErrOut:  &errBuf,
		AppName: "test",
		Env:     env,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(outBuf.String(), "yes") {
		t.Errorf("REPL did not see caller-registered global, got:\n%s", outBuf.String())
	}

	// Env should still be usable after Run returns.
	if err := env.L.DoString(`assert(hello_from_caller == "yes")`); err != nil {
		t.Errorf("env unusable after Run: %v", err)
	}
}

func TestTrimTraceback(t *testing.T) {
	in := "some error\nstack traceback:\n\t[G]: in ?"
	got := trimTraceback(in)

	if strings.Contains(got, "traceback") {
		t.Errorf("traceback not trimmed: %q", got)
	}

	if !strings.Contains(got, "some error") {
		t.Errorf("original message dropped: %q", got)
	}
}
