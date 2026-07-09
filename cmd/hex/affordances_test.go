package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -- write: action recording + dry-run -----------------------------------

func TestGeneratorWrite_dryRunRecordsWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "new.txt")

	g := newGenerator()
	g.dryRun = true

	if err := g.write(target, []byte("hi")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := os.Stat(target); err == nil {
		t.Errorf("dry-run wrote the file to disk")
	}

	if len(g.actions) != 1 || g.actions[0].Kind != "create" {
		t.Errorf("expected one create action, got %+v", g.actions)
	}
}

func TestGeneratorWrite_recordsCreateThenOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "f.txt")

	g := newGenerator()
	g.force = true

	if err := g.write(target, []byte("a")); err != nil {
		t.Fatalf("first write: %v", err)
	}

	if err := g.write(target, []byte("b")); err != nil {
		t.Fatalf("second write: %v", err)
	}

	if len(g.actions) != 2 {
		t.Fatalf("want 2 actions, got %+v", g.actions)
	}

	if g.actions[0].Kind != "create" || g.actions[1].Kind != "overwrite" {
		t.Errorf("want create then overwrite, got %q, %q", g.actions[0].Kind, g.actions[1].Kind)
	}
}

// -- wireMarker: dry-run + recording --------------------------------------

func TestGeneratorWireMarker_dryRunDoesNotModify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.go")
	src := "x\n\t// hex:providers\ny\n"

	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	g := newGenerator()
	g.dryRun = true

	if err := g.wireMarker(path, "// hex:providers", "&Payments{},", "added Payments"); err != nil {
		t.Fatalf("wireMarker: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != src {
		t.Errorf("dry-run modified the file:\n%s", got)
	}

	if len(g.actions) != 1 || g.actions[0].Kind != "wire" {
		t.Errorf("expected one wire action, got %+v", g.actions)
	}
}

func TestGeneratorWireMarker_missingMarkerErrorsInDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	_ = os.WriteFile(path, []byte("no marker here\n"), 0o644)

	g := newGenerator()
	g.dryRun = true

	if err := g.wireMarker(path, "// hex:providers", "&X{},", ""); err == nil {
		t.Errorf("expected an error for a missing marker in dry-run")
	}
}

func TestGeneratorWireMarker_recordsSkipWhenAlreadyWired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.go")
	_ = os.WriteFile(path, []byte("&Payments{},\n// hex:providers\n"), 0o644)

	g := newGenerator()

	if err := g.wireMarker(path, "// hex:providers", "&Payments{},", "added Payments"); err != nil {
		t.Fatalf("wireMarker: %v", err)
	}

	if len(g.actions) != 1 || g.actions[0].Kind != "skip" {
		t.Errorf("expected one skip action, got %+v", g.actions)
	}
}

// -- report: json output --------------------------------------------------

func TestGeneratorReport_json(t *testing.T) {
	out, err := os.CreateTemp(t.TempDir(), "report-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	g := newGenerator()
	g.format = "json"
	g.dryRun = true
	g.out = out
	g.record("create", "a.go", "")
	g.record("wire", "boot.go", "added A")

	if err := g.report(); err != nil {
		t.Fatalf("report: %v", err)
	}

	data, _ := os.ReadFile(out.Name())

	var got struct {
		DryRun  bool     `json:"dry_run"`
		Actions []action `json:"actions"`
	}

	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("report output is not valid JSON: %v\n%s", err, data)
	}

	if !got.DryRun || len(got.Actions) != 2 {
		t.Errorf("unexpected report payload: %+v", got)
	}

	if got.Actions[1].Kind != "wire" || got.Actions[1].Detail != "added A" {
		t.Errorf("wire action not preserved: %+v", got.Actions[1])
	}
}

// -- flag parsing ---------------------------------------------------------

func TestNewGeneratorFromFlags_rejectsUnknownFormat(t *testing.T) {
	if _, err := newGeneratorFromFlags(genFlags{format: "yaml"}); err == nil {
		t.Errorf("expected an error for --format yaml")
	}
}

func TestNewGeneratorFromFlags_defaultsToText(t *testing.T) {
	g, err := newGeneratorFromFlags(genFlags{})
	if err != nil {
		t.Fatalf("newGeneratorFromFlags: %v", err)
	}

	if g.format != "text" {
		t.Errorf("format = %q, want text", g.format)
	}
}

// -- embedded help --------------------------------------------------------

func TestHelp_everyCommandHasLongAndExample(t *testing.T) {
	keys := []string{
		"init", "publish", "run", "repl",
		"make_provider", "make_domain", "make_migration",
		"make_command", "make_adapter", "make_controller",
	}

	for _, k := range keys {
		if strings.TrimSpace(helpLong(k)) == "" {
			t.Errorf("%s.long.md is empty", k)
		}

		if strings.TrimSpace(helpExample(k)) == "" {
			t.Errorf("%s.example.sh is empty", k)
		}
	}
}

// -- AGENTS.md scaffold template ------------------------------------------

func TestAgentsTemplate_reflectsChosenComponents(t *testing.T) {
	render := func(cfg initConfig) string {
		g := newGenerator()
		target := filepath.Join(t.TempDir(), "AGENTS.md")

		if err := g.render("templates/init/AGENTS.md.tmpl", target, cfg); err != nil {
			t.Fatalf("render: %v", err)
		}

		data, _ := os.ReadFile(target)

		return string(data)
	}

	full := render(initConfig{Name: "full", ModulePath: "example.com/full", Web: true, Dialect: "sqlite"})
	for _, want := range []string{"make:controller", "hex:routes", "make:migration", "go run . serve"} {
		if !strings.Contains(full, want) {
			t.Errorf("web+db AGENTS.md should mention %q", want)
		}
	}

	min := render(initConfig{Name: "min", ModulePath: "example.com/min", Dialect: "none"})
	for _, absent := range []string{"make:controller", "hex:routes", "make:migration", "go run . serve"} {
		if strings.Contains(min, absent) {
			t.Errorf("no-web/no-db AGENTS.md should omit %q", absent)
		}
	}

	if !strings.Contains(min, "make:provider") {
		t.Error("every AGENTS.md should mention make:provider")
	}
}

// -- generated manpage ----------------------------------------------------

func TestRenderHex1_includesEveryVisibleCommand(t *testing.T) {
	page, err := renderHex1()
	if err != nil {
		t.Fatalf("renderHex1: %v", err)
	}

	for _, want := range []string{
		"# COMMANDS", "# SEE ALSO",
		"## hex init", "## hex make:provider", "## hex make:controller",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("hex.1 markdown missing %q", want)
		}
	}

	if strings.Contains(page, "gen-man") {
		t.Error("hidden gen-man command leaked into the generated manpage")
	}
}

// TestHelp_rootBuildsWithAllHelpFiles ensures the full command tree can be
// constructed — mustHelp panics if any embedded help file is missing, so a
// clean newRoot() proves every command's help content is present.
func TestHelp_rootBuildsWithAllHelpFiles(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("newRoot panicked (missing help file?): %v", r)
		}
	}()

	if newRoot() == nil {
		t.Fatal("newRoot returned nil")
	}
}
