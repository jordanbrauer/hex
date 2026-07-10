package init

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
	"github.com/jordanbrauer/hex/cmd/hex/infrastructure/embedfs"
)

func TestAgentsTemplate_reflectsChosenComponents(t *testing.T) {
	svc := generator.NewService(embedfs.New())

	render := func(cfg initConfig) string {
		target := filepath.Join(t.TempDir(), "AGENTS.md")

		if _, err := svc.RenderFile(context.Background(), "templates/init/AGENTS.md.tmpl", target, cfg, generator.Options{}); err != nil {
			t.Fatalf("RenderFile: %v", err)
		}

		data, _ := os.ReadFile(target)

		return string(data)
	}

	full := render(initConfig{Name: "full", ModulePath: "example.com/full", Web: true, Dialect: "sqlite"})
	for _, want := range []string{"make controller", "hex:routes", "make migration", "go run . serve"} {
		if !strings.Contains(full, want) {
			t.Errorf("web+db AGENTS.md should mention %q", want)
		}
	}

	min := render(initConfig{Name: "min", ModulePath: "example.com/min", Dialect: "none"})
	for _, absent := range []string{"make controller", "hex:routes", "make migration", "go run . serve"} {
		if strings.Contains(min, absent) {
			t.Errorf("no-web/no-db AGENTS.md should omit %q", absent)
		}
	}

	if !strings.Contains(min, "make provider") {
		t.Error("every AGENTS.md should mention make provider")
	}
}
