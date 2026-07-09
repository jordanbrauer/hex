package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRepl_expressionsPrintValues(t *testing.T) {
	in := strings.NewReader("1 + 2\n\"hello\"\nexit\n")
	var out, errOut bytes.Buffer

	if err := runRepl(in, &out, &errOut, false); err != nil {
		t.Fatalf("runRepl: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "3") {
		t.Errorf("expected \"3\" in output, got:\n%s", got)
	}

	if !strings.Contains(got, "hello") {
		t.Errorf("expected \"hello\" in output, got:\n%s", got)
	}
}

func TestRepl_statementsPersistGlobals(t *testing.T) {
	// Locals do NOT persist across REPL lines (standard Lua behavior),
	// but globals DO. Assign to a global and reference it on a later
	// line.
	in := strings.NewReader("counter = 41\ncounter + 1\nexit\n")
	var out, errOut bytes.Buffer

	if err := runRepl(in, &out, &errOut, false); err != nil {
		t.Fatalf("runRepl: %v", err)
	}

	if !strings.Contains(out.String(), "42") {
		t.Errorf("expected 42 in output, got:\n%s", out.String())
	}
}

func TestRepl_errorDoesNotAbort(t *testing.T) {
	in := strings.NewReader("error(\"boom\")\n\"still alive\"\nexit\n")
	var out, errOut bytes.Buffer

	if err := runRepl(in, &out, &errOut, false); err != nil {
		t.Fatalf("runRepl: %v", err)
	}

	if !strings.Contains(errOut.String(), "boom") {
		t.Errorf("expected error message with 'boom' on stderr, got:\n%s", errOut.String())
	}

	if strings.Contains(errOut.String(), "stack traceback") {
		t.Errorf("stderr should not include stack traceback in REPL mode:\n%s", errOut.String())
	}

	if !strings.Contains(out.String(), "still alive") {
		t.Errorf("REPL did not continue after error:\n%s", out.String())
	}
}

func TestRepl_exitDirectives(t *testing.T) {
	for _, kw := range []string{"exit", "quit", ".exit", ".quit"} {
		t.Run(kw, func(t *testing.T) {
			in := strings.NewReader("1\n" + kw + "\n2\n")
			var out, errOut bytes.Buffer

			if err := runRepl(in, &out, &errOut, false); err != nil {
				t.Fatalf("runRepl: %v", err)
			}

			if !strings.Contains(out.String(), "1") {
				t.Errorf("REPL exited before evaluating first line")
			}

			if strings.Contains(out.String(), "2") {
				t.Errorf("REPL evaluated line after %q; should have exited", kw)
			}
		})
	}
}

func TestRepl_eofExits(t *testing.T) {
	in := strings.NewReader("42\n")
	var out, errOut bytes.Buffer

	if err := runRepl(in, &out, &errOut, false); err != nil {
		t.Fatalf("runRepl EOF exit: %v", err)
	}

	if !strings.Contains(out.String(), "42") {
		t.Errorf("expected 42 in output before EOF:\n%s", out.String())
	}
}

func TestRepl_tealModeEvaluatesTypedExpressions(t *testing.T) {
	in := strings.NewReader("1 + 2\nexit\n")
	var out, errOut bytes.Buffer

	if err := runRepl(in, &out, &errOut, true); err != nil {
		t.Fatalf("runRepl teal: %v", err)
	}

	if !strings.Contains(out.String(), "3") {
		t.Errorf("expected 3 in output, got:\n%s", out.String())
	}

	if !strings.Contains(out.String(), "teal") {
		t.Errorf("banner did not identify teal mode:\n%s", out.String())
	}
}

func TestTrimTraceback(t *testing.T) {
	in := `some error\nstack traceback:\n\t[G]: in ?`
	// Passing literal `\n` above — replace so this reflects an actual multiline string.
	in = strings.ReplaceAll(in, `\n`, "\n")
	in = strings.ReplaceAll(in, `\t`, "\t")

	got := trimTraceback(in)
	if strings.Contains(got, "traceback") {
		t.Errorf("traceback not trimmed: %q", got)
	}

	if !strings.Contains(got, "some error") {
		t.Errorf("original message dropped: %q", got)
	}
}
