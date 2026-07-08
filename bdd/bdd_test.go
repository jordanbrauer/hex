package bdd_test

import (
	"embed"
	"io/fs"
	"testing"

	"github.com/jordanbrauer/hex/bdd"
)

//go:embed testdata/features/*.feature
var featuresFS embed.FS

// -- shared step definitions ---------------------------------------------

const counterKey = "counter"

func startCounter(t bdd.StepTest, ctx bdd.Context, initial int) {
	ctx.Set(counterKey, initial)
}

func addToCounter(t bdd.StepTest, ctx bdd.Context, delta int) {
	current, err := ctx.GetInt(counterKey)
	if err != nil {
		t.Fatalf("counter: %v", err)
	}

	ctx.Set(counterKey, current+delta)
}

func subtractFromCounter(t bdd.StepTest, ctx bdd.Context, delta int) {
	current, err := ctx.GetInt(counterKey)
	if err != nil {
		t.Fatalf("counter: %v", err)
	}

	ctx.Set(counterKey, current-delta)
}

func assertCounter(t bdd.StepTest, ctx bdd.Context, expected int) {
	got, err := ctx.GetInt(counterKey)
	if err != nil {
		t.Fatalf("counter: %v", err)
	}

	if got != expected {
		t.Fatalf("counter = %d, want %d", got, expected)
	}
}

func registerCounterSteps(suite *bdd.Suite) {
	suite.AddStep(`a counter starting at (\d+)`, startCounter)
	suite.AddStep(`I add (\d+)`, addToCounter)
	suite.AddStep(`I subtract (\d+)`, subtractFromCounter)
	suite.AddStep(`the counter is (\d+)`, assertCounter)
}

// -- tests ----------------------------------------------------------------

func TestNewSuiteFS_runsScenariosFromEmbed(t *testing.T) {
	suite := bdd.NewSuiteFS(t, featuresFS, "testdata/features/*.feature")

	registerCounterSteps(suite)

	suite.Run()
}

func TestNewSuiteFS_defaultGlob(t *testing.T) {
	// The default glob is "features/*.feature". fs.Sub rebases the
	// embedded FS so that glob hits our fixture directory.
	sub, err := fs.Sub(featuresFS, "testdata")
	if err != nil {
		t.Fatal(err)
	}

	suite := bdd.NewSuiteFS(t, sub, "")
	registerCounterSteps(suite)

	suite.Run()
}

func TestNewSuiteFS_noMatchesFails(t *testing.T) {
	fake := &captureT{TB: t}

	_ = bdd.NewSuiteFS(fake, featuresFS, "testdata/features/*.nomatch")

	if !fake.fatal {
		t.Errorf("expected Fatalf when no features matched")
	}
}

func TestWithTags_filtersScenarios(t *testing.T) {
	// The counter feature has no tags, so a tag filter should include
	// nothing and the suite should still run cleanly (0 scenarios).
	suite := bdd.NewSuiteFS(t, featuresFS, "testdata/features/*.feature",
		bdd.WithTags("nonexistent-tag"))

	registerCounterSteps(suite)

	suite.Run()
}

// -- helpers --------------------------------------------------------------

// captureT is a minimal *testing.T stand-in that records Fatalf calls
// without terminating the outer test. Used to assert error paths.
type captureT struct {
	testing.TB

	fatal    bool
	fatalMsg string
}

func (c *captureT) Helper()                                    {}
func (c *captureT) Fatalf(format string, args ...any)          { c.fatal = true }
func (c *captureT) Errorf(format string, args ...any)          {}
func (c *captureT) Fail()                                      {}
func (c *captureT) FailNow()                                   {}
func (c *captureT) Log(args ...any)                            {}
func (c *captureT) Logf(format string, args ...any)            {}
func (c *captureT) TempDir() string                            { return c.TB.TempDir() }
func (c *captureT) Parallel()                                  {}
func (c *captureT) Run(name string, f func(t *testing.T)) bool { return true }
