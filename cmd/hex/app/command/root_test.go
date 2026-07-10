package command

import (
	"testing"

	"github.com/jordanbrauer/hex"
)

// TestRoot_buildsWithAllHelpFiles ensures the full command tree can be
// constructed — mustHelp panics if any embedded help file is missing, so
// a clean Root() proves every command's help content is present.
func TestRoot_buildsWithAllHelpFiles(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Root panicked (missing help file?): %v", r)
		}
	}()

	if Root(hex.New()) == nil {
		t.Fatal("Root returned nil")
	}
}
