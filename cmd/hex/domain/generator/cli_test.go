package generator

import (
	"os"
	"path/filepath"
	"testing"
)

// -- ProjectRoot / parseModulePath ---------------------------------------

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

	root, mod, err := ProjectRoot()
	if err != nil {
		t.Fatalf("ProjectRoot: %v", err)
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

	if _, _, err := ProjectRoot(); err == nil {
		t.Errorf("expected error when no go.mod is present")
	}
}

// -- Flags.Options ----------------------------------------------------------

func TestFlagsOptions_rejectsUnknownFormat(t *testing.T) {
	if _, err := (Flags{Format: "yaml"}).Options(); err == nil {
		t.Errorf("expected an error for --format yaml")
	}
}

func TestFlagsOptions_defaultsToText(t *testing.T) {
	f := Flags{}
	if _, err := f.Options(); err != nil {
		t.Fatalf("Options: %v", err)
	}
}
