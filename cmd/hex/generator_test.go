package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -- projectRoot / parseModulePath ---------------------------------------

func TestParseModulePath(t *testing.T) {
	src := `module github.com/example/app

go 1.26

require github.com/jordanbrauer/hex v0.0.0
`

	got, err := parseModulePath([]byte(src))
	if err != nil {
		t.Fatalf("parseModulePath: %v", err)
	}

	if got != "github.com/example/app" {
		t.Errorf("got %q", got)
	}
}

func TestParseModulePath_missing(t *testing.T) {
	if _, err := parseModulePath([]byte("go 1.26\n")); err == nil {
		t.Errorf("expected error for missing module directive")
	}
}

func TestProjectRoot_findsGoMod(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")

	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	gomod := "module example.com/x\n\ngo 1.26\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run from the deepest directory.
	orig, _ := os.Getwd()

	defer os.Chdir(orig)

	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}

	root, mod, err := projectRoot()
	if err != nil {
		t.Fatalf("projectRoot: %v", err)
	}

	// Resolve symlinks (macOS /var → /private/var) on both sides.
	wantRoot, _ := filepath.EvalSymlinks(dir)
	gotRoot, _ := filepath.EvalSymlinks(root)

	if wantRoot != gotRoot {
		t.Errorf("root = %q, want %q", gotRoot, wantRoot)
	}

	if mod != "example.com/x" {
		t.Errorf("module = %q, want example.com/x", mod)
	}
}

func TestProjectRoot_notFound(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()

	defer os.Chdir(orig)

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	if _, _, err := projectRoot(); err == nil {
		t.Errorf("expected error when no go.mod is present")
	}
}

// -- insertBeforeMarker --------------------------------------------------

func TestInsertBeforeMarker(t *testing.T) {
	src := `package provider

func Boot(app *hex.App) error {
	return app.Register(
		&Database{},
		// hex:providers
	)
}
`

	dir := t.TempDir()
	path := filepath.Join(dir, "boot.go")

	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := insertBeforeMarker(path, "// hex:providers", "&Payments{},"); err != nil {
		t.Fatalf("insertBeforeMarker: %v", err)
	}

	got, _ := os.ReadFile(path)

	wantContains := "&Payments{},"
	if !strings.Contains(string(got), wantContains) {
		t.Errorf("file missing %q\n---\n%s", wantContains, got)
	}

	// The insertion must appear above the marker.
	idxIns := strings.Index(string(got), "&Payments{},")
	idxMk := strings.Index(string(got), "// hex:providers")

	if idxIns >= idxMk {
		t.Errorf("insertion at %d not before marker at %d", idxIns, idxMk)
	}
}

func TestInsertBeforeMarker_idempotent(t *testing.T) {
	src := `x
&Payments{},
// hex:providers
y
`
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	_ = os.WriteFile(path, []byte(src), 0o644)

	// The insertion is already there; calling again should not duplicate.
	if err := insertBeforeMarker(path, "// hex:providers", "&Payments{},"); err != nil {
		t.Fatalf("insertBeforeMarker: %v", err)
	}

	got, _ := os.ReadFile(path)
	if strings.Count(string(got), "&Payments{},") != 1 {
		t.Errorf("insertion duplicated:\n%s", got)
	}
}

func TestInsertBeforeMarker_missingMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	_ = os.WriteFile(path, []byte("nothing here\n"), 0o644)

	if err := insertBeforeMarker(path, "// hex:providers", "&X{},"); err == nil {
		t.Errorf("expected error for missing marker")
	}
}

// -- generator render ----------------------------------------------------

func TestGenerator_refusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "existing.txt")
	_ = os.WriteFile(target, []byte("original"), 0o644)

	g := newGenerator()
	err := g.write(target, []byte("new"))

	if err == nil {
		t.Errorf("write should refuse to overwrite without --force")
	}

	got, _ := os.ReadFile(target)
	if string(got) != "original" {
		t.Errorf("file was overwritten: %q", got)
	}
}

func TestGenerator_forceOverwrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "existing.txt")
	_ = os.WriteFile(target, []byte("original"), 0o644)

	g := newGenerator()
	g.force = true

	if err := g.write(target, []byte("new")); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("file not overwritten: %q", got)
	}
}
