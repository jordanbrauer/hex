// Package bdd wraps github.com/go-bdd/gobdd so hex applications can write
// behavior-driven tests with standard Gherkin `.feature` files.
//
// The wrapper adds:
//
//   - Type aliases so consumers get the full gobdd API through hex.
//   - Option re-exports for common configuration.
//   - NewSuite for the typical on-disk feature-file setup.
//   - NewSuiteFS which materialises `//go:embed`-ed features into a temp
//     directory at test time, then points gobdd at that directory.
//
// See ADR-0015.
//
// Example (embed.FS):
//
//	//go:embed features/*.feature
//	var featuresFS embed.FS
//
//	func TestFeatures(t *testing.T) {
//	    suite := bdd.NewSuiteFS(t, featuresFS, "features/*.feature")
//	    suite.AddStep(`I have (\d+) items`, hasItems)
//	    suite.AddStep(`I remove (\d+) items`, removeItems)
//	    suite.Run()
//	}
package bdd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-bdd/gobdd"
)

// Suite is the type alias for gobdd's test suite.
type Suite = gobdd.Suite

// SuiteOptions is the type alias for gobdd's option bag.
type SuiteOptions = gobdd.SuiteOptions

// Context is the type alias for the per-scenario Context passed to step
// functions and lifecycle hooks.
type Context = gobdd.Context

// TestingT is the *testing.T subset gobdd requires. hex re-exports it so
// consumers can accept it in their own helpers without importing gobdd.
type TestingT = gobdd.TestingT

// StepTest is the interface for gobdd's step-level assertions.
type StepTest = gobdd.StepTest

// Option is a functional option for the suite.
type Option = func(*gobdd.SuiteOptions)

// -- Re-exported options --------------------------------------------------

// RunInParallel enables parallel scenario execution.
func RunInParallel() Option { return gobdd.RunInParallel() }

// WithFeaturesPath overrides the default `features/*.feature` glob with
// path. Useful when features live outside the conventional directory.
func WithFeaturesPath(path string) Option { return gobdd.WithFeaturesPath(path) }

// WithTags restricts execution to scenarios carrying any of the given tags.
func WithTags(tags ...string) Option { return gobdd.WithTags(tags...) }

// WithIgnoredTags skips scenarios carrying any of the given tags.
func WithIgnoredTags(tags ...string) Option { return gobdd.WithIgnoredTags(tags...) }

// WithBeforeScenario registers a hook run before each scenario.
func WithBeforeScenario(fn func(Context)) Option { return gobdd.WithBeforeScenario(fn) }

// WithAfterScenario registers a hook run after each scenario.
func WithAfterScenario(fn func(Context)) Option { return gobdd.WithAfterScenario(fn) }

// WithBeforeStep registers a hook run before each step.
func WithBeforeStep(fn func(Context)) Option { return gobdd.WithBeforeStep(fn) }

// WithAfterStep registers a hook run after each step.
func WithAfterStep(fn func(Context)) Option { return gobdd.WithAfterStep(fn) }

// -- Constructors ---------------------------------------------------------

// NewSuite is a passthrough constructor for the on-disk case. Equivalent
// to gobdd.NewSuite but returns the hex-aliased type.
func NewSuite(t TestingT, opts ...Option) *Suite {
	return gobdd.NewSuite(t, opts...)
}

// NewSuiteFS materialises feature files matching glob from f into a
// temporary directory and returns a Suite pointing at that directory.
// The temp directory is cleaned up when t completes.
//
// Use this when feature files are embedded via //go:embed so the tests
// can run against a fully compiled binary without carrying the feature
// files on disk at runtime.
//
// glob is a fs.Glob pattern relative to f's root. Passing "" defaults to
// "features/*.feature".
func NewSuiteFS(t TestingTB, f fs.FS, glob string, opts ...Option) *Suite {
	t.Helper()

	if glob == "" {
		glob = "features/*.feature"
	}

	tmp := t.TempDir()

	if err := materialiseFeatures(f, glob, tmp); err != nil {
		t.Fatalf("bdd: materialise features: %v", err)
	}

	opts = append([]Option{gobdd.WithFeaturesPath(filepath.Join(tmp, filepath.Base(glob)))}, opts...)

	return gobdd.NewSuite(t, opts...)
}

// TestingTB is the subset of *testing.T that NewSuiteFS uses. Widened
// beyond gobdd.TestingT so we can call TempDir and Helper. Standard
// *testing.T satisfies it.
type TestingTB interface {
	TestingT
	Helper()
	TempDir() string
	Fatalf(format string, args ...any)
}

// materialiseFeatures walks f matching glob and writes each file into
// dst preserving the leaf filename.
func materialiseFeatures(f fs.FS, glob, dst string) error {
	matches, err := fs.Glob(f, glob)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		return errors.New("bdd: no feature files matched glob " + glob)
	}

	for _, path := range matches {
		data, err := fs.ReadFile(f, path)
		if err != nil {
			return err
		}

		name := filepath.Base(path)
		if !strings.HasSuffix(name, ".feature") {
			continue
		}

		if err := os.WriteFile(filepath.Join(dst, name), data, 0o600); err != nil {
			return err
		}
	}

	return nil
}
