package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	hexcli "github.com/jordanbrauer/hex/cli"
	hexlog "github.com/jordanbrauer/hex/log"
)

func newRoot(t *testing.T) (*cobra.Command, *hex.App, *bytes.Buffer) {
	t.Helper()

	app := hex.New()
	root := hexcli.Root(hexcli.RootOptions{
		Name:  "myapp",
		Short: "test app",
		App:   app,
	})

	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)

	return root, app, buf
}

func TestRoot_basicMetadata(t *testing.T) {
	root, _, _ := newRoot(t)

	if root.Use != "myapp" {
		t.Errorf("Use = %q, want myapp", root.Use)
	}

	if !root.SilenceUsage {
		t.Errorf("SilenceUsage = false, want true (default)")
	}

	if !root.SilenceErrors {
		t.Errorf("SilenceErrors = false, want true (default)")
	}
}

func TestRoot_enableFlagsInvert(t *testing.T) {
	root := hexcli.Root(hexcli.RootOptions{
		Name:               "myapp",
		EnableUsageOnError: true,
		EnableErrorPrint:   true,
	})

	if root.SilenceUsage {
		t.Errorf("SilenceUsage = true with EnableUsageOnError=true")
	}

	if root.SilenceErrors {
		t.Errorf("SilenceErrors = true with EnableErrorPrint=true")
	}
}

func TestRoot_installsPersistentFlags(t *testing.T) {
	root, _, _ := newRoot(t)

	for _, name := range []string{"log-level", "env", "verbose"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("persistent flag %q missing", name)
		}
	}
}

func TestRoot_stashesAppInContext(t *testing.T) {
	root, app, _ := newRoot(t)

	got := hexcli.FromContext(root.Context())
	if got != app {
		t.Errorf("FromContext = %p, want %p", got, app)
	}
}

func TestFromContext_nilCtxAndNoAppReturnsNil(t *testing.T) {
	if got := hexcli.FromContext(nil); got != nil {
		t.Errorf("FromContext(nil) = %v, want nil", got)
	}

	if got := hexcli.FromContext(context.Background()); got != nil {
		t.Errorf("FromContext(empty ctx) = %v, want nil", got)
	}
}

func TestPersistentPreRun_logLevelApplied(t *testing.T) {
	t.Cleanup(func() { hexlog.SetLevel(hexlog.FatalLevel) })

	root, _, _ := newRoot(t)

	// Give the root a runnable RunE so Execute doesn't just print help.
	root.RunE = func(*cobra.Command, []string) error { return nil }
	root.SetArgs([]string{"--log-level", "warn"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error = %v", err)
	}

	if got := hexlog.GetLevel(); got != hexlog.WarnLevel {
		t.Errorf("level after --log-level=warn = %v, want warn", got)
	}
}

func TestPersistentPreRun_verboseWins(t *testing.T) {
	t.Cleanup(func() { hexlog.SetLevel(hexlog.FatalLevel) })

	root, _, _ := newRoot(t)
	root.RunE = func(*cobra.Command, []string) error { return nil }
	root.SetArgs([]string{"--log-level", "warn", "--verbose"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error = %v", err)
	}

	if got := hexlog.GetLevel(); got != hexlog.DebugLevel {
		t.Errorf("level with --verbose = %v, want debug", got)
	}
}

func TestPersistentPreRun_invalidLevelErrors(t *testing.T) {
	root, _, _ := newRoot(t)
	root.RunE = func(*cobra.Command, []string) error { return nil }
	root.SetArgs([]string{"--log-level", "chatty"})

	err := root.Execute()
	if err == nil {
		t.Fatal("Execute with bad log level returned nil error")
	}

	if !strings.Contains(err.Error(), "invalid --log-level") {
		t.Errorf("error = %v, want contains 'invalid --log-level'", err)
	}
}

func TestPersistentPreRun_emptyLevelIsNoop(t *testing.T) {
	t.Cleanup(func() { hexlog.SetLevel(hexlog.FatalLevel) })

	hexlog.SetLevel(hexlog.InfoLevel)

	root, _, _ := newRoot(t)
	root.RunE = func(*cobra.Command, []string) error { return nil }
	root.SetArgs([]string{})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error = %v", err)
	}

	if got := hexlog.GetLevel(); got != hexlog.InfoLevel {
		t.Errorf("level with no flag = %v, want unchanged info", got)
	}
}

func TestWithPreRun_runsAfterHexes(t *testing.T) {
	t.Cleanup(func() { hexlog.SetLevel(hexlog.FatalLevel) })

	root, _, _ := newRoot(t)

	saw := ""
	hexcli.WithPreRun(root, func(*cobra.Command, []string) error {
		saw = hexlog.GetLevel().String()

		return nil
	})

	root.RunE = func(*cobra.Command, []string) error { return nil }
	root.SetArgs([]string{"--log-level", "error"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error = %v", err)
	}

	if saw != "error" {
		t.Errorf("consumer PreRun saw level %q; want error (proves hex's ran first)", saw)
	}
}

func TestWithPreRun_hexErrorSkipsConsumer(t *testing.T) {
	root, _, _ := newRoot(t)

	called := false
	hexcli.WithPreRun(root, func(*cobra.Command, []string) error {
		called = true

		return nil
	})

	root.RunE = func(*cobra.Command, []string) error { return nil }
	root.SetArgs([]string{"--log-level", "chatty"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error")
	}

	if called {
		t.Errorf("consumer PreRun ran despite hex's returning an error")
	}
}

func TestExecute_returnsZeroOnSuccess(t *testing.T) {
	root, _, _ := newRoot(t)
	root.RunE = func(*cobra.Command, []string) error { return nil }
	root.SetArgs([]string{})

	if code := hexcli.Execute(root); code != 0 {
		t.Errorf("Execute() = %d, want 0", code)
	}
}

func TestExecute_returnsOneOnError(t *testing.T) {
	root, _, _ := newRoot(t)
	root.RunE = func(*cobra.Command, []string) error { return errors.New("kaboom") }
	root.SetArgs([]string{})

	if code := hexcli.Execute(root); code != 1 {
		t.Errorf("Execute() = %d, want 1", code)
	}
}

func TestVersion_shortForm(t *testing.T) {
	root, _, buf := newRoot(t)

	v := hexcli.Version(hexcli.VersionOptions{App: "myapp"})
	root.AddCommand(v)

	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error = %v", err)
	}

	out := buf.String()
	if !strings.HasPrefix(out, "myapp ") {
		t.Errorf("version output = %q, want prefix 'myapp '", out)
	}
}

func TestVersion_longFlagPrintsInfo(t *testing.T) {
	root, _, buf := newRoot(t)
	root.AddCommand(hexcli.Version(hexcli.VersionOptions{App: "myapp"}))

	root.SetArgs([]string{"version", "--long"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error = %v", err)
	}

	out := buf.String()

	for _, want := range []string{"version:", "commit:", "platform:"} {
		if !strings.Contains(out, want) {
			t.Errorf("version --long missing %q\n---\n%s", want, out)
		}
	}
}

func TestVersion_fallsBackToRootUse(t *testing.T) {
	root, _, buf := newRoot(t)
	root.AddCommand(hexcli.Version(hexcli.VersionOptions{}))

	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error = %v", err)
	}

	if !strings.HasPrefix(buf.String(), "myapp ") {
		t.Errorf("expected fallback to root.Use, got %q", buf.String())
	}
}
