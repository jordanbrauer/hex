package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// -- writeFile ------------------------------------------------------------

func TestWriteFile_refusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "existing.txt")
	_ = os.WriteFile(target, []byte("original"), 0o644)

	_, err := writeFile(target, []byte("new"), Options{})
	if err == nil {
		t.Errorf("write should refuse to overwrite without Force")
	}

	got, _ := os.ReadFile(target)
	if string(got) != "original" {
		t.Errorf("file was overwritten: %q", got)
	}
}

func TestWriteFile_forceOverwrites(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "existing.txt")
	_ = os.WriteFile(target, []byte("original"), 0o644)

	if _, err := writeFile(target, []byte("new"), Options{Force: true}); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("file not overwritten: %q", got)
	}
}

func TestWriteFile_dryRunRecordsWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "new.txt")

	act, err := writeFile(target, []byte("hi"), Options{DryRun: true})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := os.Stat(target); err == nil {
		t.Errorf("dry-run wrote the file to disk")
	}

	if act.Kind != "create" {
		t.Errorf("expected a create action, got %+v", act)
	}
}

func TestWriteFile_recordsCreateThenOverwrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "f.txt")

	act1, err := writeFile(target, []byte("a"), Options{Force: true})
	if err != nil {
		t.Fatalf("first write: %v", err)
	}

	act2, err := writeFile(target, []byte("b"), Options{Force: true})
	if err != nil {
		t.Fatalf("second write: %v", err)
	}

	if act1.Kind != "create" || act2.Kind != "overwrite" {
		t.Errorf("want create then overwrite, got %q, %q", act1.Kind, act2.Kind)
	}
}

// -- Service.WireMarker: dry-run + recording ------------------------------

func TestServiceWireMarker_dryRunDoesNotModify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.go")
	src := "x\n\t// hex:providers\ny\n"

	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(nil)

	act, err := svc.WireMarker(path, "// hex:providers", "&Payments{},", "added Payments", Options{DryRun: true})
	if err != nil {
		t.Fatalf("WireMarker: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != src {
		t.Errorf("dry-run modified the file:\n%s", got)
	}

	if act == nil || act.Kind != "wire" {
		t.Errorf("expected a wire action, got %+v", act)
	}
}

func TestServiceWireMarker_missingMarkerErrorsInDryRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	_ = os.WriteFile(path, []byte("no marker here\n"), 0o644)

	svc := NewService(nil)

	if _, err := svc.WireMarker(path, "// hex:providers", "&X{},", "", Options{DryRun: true}); err == nil {
		t.Errorf("expected an error for a missing marker in dry-run")
	}
}

func TestServiceWireMarker_recordsSkipWhenAlreadyWired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot.go")
	_ = os.WriteFile(path, []byte("&Payments{},\n// hex:providers\n"), 0o644)

	svc := NewService(nil)

	act, err := svc.WireMarker(path, "// hex:providers", "&Payments{},", "added Payments", Options{})
	if err != nil {
		t.Fatalf("WireMarker: %v", err)
	}

	if act == nil || act.Kind != "skip" {
		t.Errorf("expected a skip action, got %+v", act)
	}
}

// -- Report: json output --------------------------------------------------

func TestReport_json(t *testing.T) {
	out, err := os.CreateTemp(t.TempDir(), "report-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	actions := []Action{
		{Kind: "create", Path: "a.go"},
		{Kind: "wire", Path: "boot.go", Detail: "added A"},
	}

	if err := Report(out, actions, true, "json"); err != nil {
		t.Fatalf("Report: %v", err)
	}

	data, _ := os.ReadFile(out.Name())

	var got struct {
		DryRun  bool     `json:"dry_run"`
		Actions []Action `json:"actions"`
	}

	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Report output is not valid JSON: %v\n%s", err, data)
	}

	if !got.DryRun || len(got.Actions) != 2 {
		t.Errorf("unexpected report payload: %+v", got)
	}

	if got.Actions[1].Kind != "wire" || got.Actions[1].Detail != "added A" {
		t.Errorf("wire action not preserved: %+v", got.Actions[1])
	}
}
