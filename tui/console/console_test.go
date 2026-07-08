package console_test

import (
	"bytes"
	"testing"

	"github.com/muesli/termenv"
	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex/tui/console"
	"github.com/jordanbrauer/hex/tui/markup"
	"github.com/jordanbrauer/hex/tui/renderer"
)

func newCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "")

	// non-interactive default so prompt tests exercise the ErrNonInteractive
	// path without a TTY.
	cmd.Flags().Bool("non-interactive", true, "")

	out := &bytes.Buffer{}
	errb := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errb)

	return cmd, out, errb
}

func TestNew_wrapsCommand(t *testing.T) {
	cmd, _, _ := newCmd()

	c := console.New(cmd)
	if c == nil {
		t.Fatal("New returned nil")
	}

	if c.Format() != renderer.FormatTable {
		t.Errorf("Format = %v, want table", c.Format())
	}
}

func TestPrint_textFormatWritesToStdout(t *testing.T) {
	markup.SetColorProfile(termenv.Ascii)

	cmd, out, errb := newCmd()
	console.New(cmd).Println("<bold>hello</bold>")

	if out.Len() == 0 {
		t.Errorf("expected output on stdout, got empty")
	}

	if errb.Len() != 0 {
		t.Errorf("expected empty stderr, got %q", errb.String())
	}
}

func TestPrint_jsonFormatWritesToStderr(t *testing.T) {
	markup.SetColorProfile(termenv.Ascii)

	cmd, out, errb := newCmd()
	_ = cmd.Flags().Set("format", "json")

	console.New(cmd).Println("<bold>hello</bold>")

	if errb.Len() == 0 {
		t.Errorf("expected output on stderr (json format), got empty")
	}

	if out.Len() != 0 {
		t.Errorf("expected empty stdout (json format), got %q", out.String())
	}
}

func TestSuccess_prefixesCheckMark(t *testing.T) {
	markup.SetColorProfile(termenv.Ascii)

	cmd, out, _ := newCmd()
	console.New(cmd).Success("all good")

	got := out.String()
	if !bytes.Contains([]byte(got), []byte("✓")) {
		t.Errorf("Success output = %q, want to contain ✓", got)
	}

	if !bytes.Contains([]byte(got), []byte("all good")) {
		t.Errorf("Success output missing message: %q", got)
	}
}

func TestPrompts_nonInteractiveReturnErr(t *testing.T) {
	// Non-interactive is set on the flag, but Console reads it from
	// cmd.Root(). To simulate the real scenario we make the cmd its own root.
	cmd, _, _ := newCmd()
	c := console.New(cmd)

	if _, err := c.Ask("name?"); err != console.ErrNonInteractive {
		t.Errorf("Ask non-interactive = %v, want ErrNonInteractive", err)
	}

	if _, err := c.Confirm("go?"); err != console.ErrNonInteractive {
		t.Errorf("Confirm non-interactive = %v, want ErrNonInteractive", err)
	}

	if _, err := c.Secret("pw?"); err != console.ErrNonInteractive {
		t.Errorf("Secret non-interactive = %v, want ErrNonInteractive", err)
	}

	if _, err := c.Choose("pick", []console.Option{{Label: "a", Value: "a"}}); err != console.ErrNonInteractive {
		t.Errorf("Choose non-interactive = %v, want ErrNonInteractive", err)
	}

	if _, err := c.Select("pick", []console.Option{{Label: "a", Value: "a"}}, nil); err != console.ErrNonInteractive {
		t.Errorf("Select non-interactive = %v, want ErrNonInteractive", err)
	}
}
