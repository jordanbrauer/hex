// Package build exposes compile-time build metadata: version, commit, branch,
// build time, and platform information.
//
// Values are provided in one of two ways, in order of precedence:
//
//  1. ldflags at link time, e.g.
//     go build -ldflags "-X github.com/jordanbrauer/hex/build.version=v1.2.3 \
//     -X github.com/jordanbrauer/hex/build.commit=abcdef  \
//     -X github.com/jordanbrauer/hex/build.date=2026-01-02T03:04:05Z"
//  2. debug.ReadBuildInfo, which the Go toolchain populates automatically for
//     module-aware builds (`go build` inside a git repo, `go install`, etc.).
//     This is how we get a real commit and build time without requiring the
//     caller to pass ldflags.
//
// When neither source has a value the accessors return safe placeholders
// ("dev", "HEAD") and Debug() reports true.
//
// This package intentionally performs no I/O beyond reading debug.BuildInfo
// once at import time. Nothing shells out to git. Nothing panics.
package build

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// The placeholder values used when nothing else is known. Exported so tests
// (and consumers who want to detect the "unknown" state) can compare against
// them without repeating string literals.
const (
	UnknownVersion = "dev"
	UnknownCommit  = "HEAD"
	UnknownBranch  = "HEAD"
)

// ldflags targets. These are string-typed and unexported so consumers can only
// read the resolved values via the accessor functions. `-X` overrides them at
// link time.
//
//nolint:gochecknoglobals // required for ldflags injection
var (
	version = ""
	commit  = ""
	branch  = ""
	date    = "" // RFC3339
)

// Resolved values. Populated once by resolve() during package init from the
// ldflags vars plus debug.BuildInfo. Kept as package-level state so accessors
// are O(1) and allocation-free.
//
//nolint:gochecknoglobals // required for accessor caching
var (
	resolvedVersion string
	resolvedCommit  string
	resolvedBranch  string
	resolvedTime    time.Time
	fromLDFlags     bool
	vcsModified     bool
)

func init() {
	resolve()
}

// resolve populates the resolvedX package vars from ldflags and, where those
// are empty, from debug.ReadBuildInfo. Called from init(); exposed as an
// unexported func so tests could re-invoke it if needed.
func resolve() {
	resolvedVersion = version
	resolvedCommit = commit
	resolvedBranch = branch

	fromLDFlags = version != "" || commit != "" || branch != "" || date != ""

	if date != "" {
		if t, err := time.Parse(time.RFC3339, date); err == nil {
			resolvedTime = t
		}
	}

	info, ok := debug.ReadBuildInfo()
	if ok {
		if resolvedVersion == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			resolvedVersion = info.Main.Version
		}

		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if resolvedCommit == "" {
					resolvedCommit = s.Value
				}
			case "vcs.time":
				if resolvedTime.IsZero() {
					if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
						resolvedTime = t
					}
				}
			case "vcs.modified":
				vcsModified = s.Value == "true"
			}
		}
	}

	if resolvedVersion == "" {
		resolvedVersion = UnknownVersion
	}

	if resolvedCommit == "" {
		resolvedCommit = UnknownCommit
	}

	if resolvedBranch == "" {
		resolvedBranch = UnknownBranch
	}
}

// Version returns the release version. Sourced from -ldflags "-X ...version"
// first, then Go module version, then "dev".
func Version() string { return resolvedVersion }

// Commit returns the full commit SHA. Sourced from -ldflags "-X ...commit"
// first, then debug.BuildInfo (vcs.revision), then "HEAD".
func Commit() string { return resolvedCommit }

// ShortCommit returns the first 7 characters of Commit, or Commit itself if
// it is shorter than 7 characters (e.g. the "HEAD" placeholder).
func ShortCommit() string {
	if len(resolvedCommit) >= 7 {
		return resolvedCommit[:7]
	}

	return resolvedCommit
}

// Branch returns the source branch. Only populated via ldflags; debug.BuildInfo
// does not expose branch information. Returns "HEAD" if unset.
func Branch() string { return resolvedBranch }

// Time returns the build time. Sourced from -ldflags "-X ...date" (RFC3339)
// first, then debug.BuildInfo (vcs.time). Returns the zero time if neither is
// available.
func Time() time.Time { return resolvedTime }

// Modified reports whether the working tree had uncommitted changes at build
// time, according to debug.BuildInfo. False for ldflags-only builds.
func Modified() bool { return vcsModified }

// GoVersion returns the Go toolchain version the binary was compiled with,
// stripped of the leading "go" (e.g. "1.26.4" not "go1.26.4").
func GoVersion() string { return strings.TrimPrefix(runtime.Version(), "go") }

// OS returns runtime.GOOS.
func OS() string { return runtime.GOOS }

// Arch returns runtime.GOARCH.
func Arch() string { return runtime.GOARCH }

// Compiler returns runtime.Compiler ("gc" or "gccgo").
func Compiler() string { return runtime.Compiler }

// Debug reports whether this is an unversioned / development build. True when
// neither ldflags nor debug.BuildInfo produced a real version or commit.
func Debug() bool {
	return !fromLDFlags &&
		(resolvedVersion == UnknownVersion || resolvedCommit == UnknownCommit)
}

// Info returns a multi-line human-readable summary of the build metadata.
// Suitable as the body of a `version` CLI subcommand.
func Info() string {
	var b strings.Builder

	fmt.Fprintf(&b, "version:  %s\n", Version())
	fmt.Fprintf(&b, "commit:   %s", Commit())
	if Modified() {
		b.WriteString(" (modified)")
	}

	b.WriteByte('\n')
	fmt.Fprintf(&b, "branch:   %s\n", Branch())

	if t := Time(); !t.IsZero() {
		fmt.Fprintf(&b, "built:    %s\n", t.Format(time.RFC3339))
	}

	fmt.Fprintf(&b, "go:       %s\n", GoVersion())
	fmt.Fprintf(&b, "platform: %s/%s\n", OS(), Arch())
	fmt.Fprintf(&b, "compiler: %s\n", Compiler())

	if Debug() {
		b.WriteString("mode:     debug\n")
	}

	return b.String()
}
