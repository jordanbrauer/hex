package init

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jordanbrauer/hex/build"
	"github.com/jordanbrauer/hex/cmd/hex/domain/generator"
	"github.com/jordanbrauer/hex/cmd/hex/infrastructure/embedfs"
)

func TestGoModuleVersion_formatsReleaseVersions(t *testing.T) {
	for _, tt := range []struct {
		name    string
		version string
		want    string
	}{
		{name: "version without prefix", version: "0.0.2-alpha", want: "v0.0.2-alpha"},
		{name: "version with prefix", version: "v0.0.2-alpha", want: "v0.0.2-alpha"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := goModuleVersion(tt.version); got != tt.want {
				t.Errorf("goModuleVersion(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestResolveInitConfig_formatsHexVersionForGoModules(t *testing.T) {
	cfg, err := resolveInitConfig([]string{"example"}, resolveFlags{yes: true})
	if err != nil {
		t.Fatalf("resolveInitConfig: %v", err)
	}

	want := goModuleVersion(build.Version())
	if cfg.HexVersion != want {
		t.Errorf("HexVersion = %q, want %q", cfg.HexVersion, want)
	}
}

func TestProviderLuaTemplate_includesPackageDocumentation(t *testing.T) {
	svc := generator.NewService(embedfs.New())
	target := filepath.Join(t.TempDir(), "lua.go")

	if _, err := svc.RenderFile(context.Background(), "templates/init/provider_lua.go.tmpl", target, initConfig{}, generator.Options{}); err != nil {
		t.Fatalf("RenderFile: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(data), "// Package provider defines the application-specific Hex service providers.") {
		t.Error("provider template should include a package documentation comment")
	}
}

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
