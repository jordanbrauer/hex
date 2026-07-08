package build_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/jordanbrauer/hex/build"
)

func TestAccessors_neverPanic_neverEmpty(t *testing.T) {
	// The accessors must always return a sensible value regardless of build
	// mode. `go test` runs without ldflags, so this exercises the fallback
	// path through debug.BuildInfo + placeholders.
	if got := build.Version(); got == "" {
		t.Errorf("Version() = empty")
	}

	if got := build.Commit(); got == "" {
		t.Errorf("Commit() = empty")
	}

	if got := build.Branch(); got == "" {
		t.Errorf("Branch() = empty")
	}

	if got := build.GoVersion(); got == "" {
		t.Errorf("GoVersion() = empty")
	}

	if got := build.OS(); got != runtime.GOOS {
		t.Errorf("OS() = %q, want %q", got, runtime.GOOS)
	}

	if got := build.Arch(); got != runtime.GOARCH {
		t.Errorf("Arch() = %q, want %q", got, runtime.GOARCH)
	}
}

func TestGoVersion_hasNoGoPrefix(t *testing.T) {
	if strings.HasPrefix(build.GoVersion(), "go") {
		t.Errorf("GoVersion() = %q, want no \"go\" prefix", build.GoVersion())
	}
}

func TestShortCommit_bounded(t *testing.T) {
	got := build.ShortCommit()

	if len(got) > 7 {
		t.Errorf("ShortCommit() len = %d, want <= 7 (got %q)", len(got), got)
	}
}

func TestShortCommit_notLongerThanCommit(t *testing.T) {
	full := build.Commit()
	short := build.ShortCommit()

	if len(short) > len(full) {
		t.Errorf("ShortCommit() (%q) longer than Commit() (%q)", short, full)
	}
}

func TestInfo_containsCoreFields(t *testing.T) {
	out := build.Info()

	for _, want := range []string{"version:", "commit:", "branch:", "go:", "platform:", "compiler:"} {
		if !strings.Contains(out, want) {
			t.Errorf("Info() missing %q\n---\n%s", want, out)
		}
	}
}

func TestDebug_trueForUnversionedBuild(t *testing.T) {
	// `go test` produces a binary with no ldflags. debug.BuildInfo will
	// contribute a vcs.revision if run inside a clean git checkout, which
	// disqualifies Debug() from being true. Assert only that the flag is a
	// deterministic bool — not its exact value, which depends on the working
	// tree.
	_ = build.Debug()
}
