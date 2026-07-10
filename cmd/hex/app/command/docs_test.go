package command

import (
	"strings"
	"testing"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/cmd/hex/app/command/genman"
)

func TestRenderHex1_includesEveryVisibleCommand(t *testing.T) {
	page, err := genman.RenderHex1(Root(hex.New()))
	if err != nil {
		t.Fatalf("RenderHex1: %v", err)
	}

	for _, want := range []string{
		"# COMMANDS", "# SEE ALSO",
		"## hex init", "## hex make provider", "## hex make controller",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("hex.1 markdown missing %q", want)
		}
	}

	if strings.Contains(page, "gen-man") {
		t.Error("hidden gen-man command leaked into the generated manpage")
	}
}
