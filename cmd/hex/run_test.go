package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveScript_inlineWins(t *testing.T) {
	src, name, isTeal, err := resolveScript(nil, "print(1)", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if src != "print(1)" {
		t.Errorf("src = %q, want print(1)", src)
	}

	if name != "<inline>" {
		t.Errorf("name = %q, want <inline>", name)
	}

	if isTeal {
		t.Errorf("isTeal true, want false without --teal")
	}
}

func TestResolveScript_inlineTeal(t *testing.T) {
	_, _, isTeal, err := resolveScript(nil, "local x: number = 1", true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !isTeal {
		t.Errorf("isTeal false, want true with --teal")
	}
}

func TestResolveScript_fileExtensionDetectsTeal(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "foo.tl")

	if err := os.WriteFile(fp, []byte("local x: number = 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	src, name, isTeal, err := resolveScript([]string{fp}, "", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !isTeal {
		t.Errorf("isTeal false for .tl file, want true")
	}

	if !strings.Contains(src, "number = 1") {
		t.Errorf("src did not include file content: %q", src)
	}

	if !strings.HasSuffix(name, "foo.tl") {
		t.Errorf("name = %q, want to end with foo.tl", name)
	}
}

func TestResolveScript_fileExtensionOverridesForceTeal(t *testing.T) {
	// A .lua file with --teal should still be treated as Lua — file
	// extensions win. --teal only affects inline and stdin.
	dir := t.TempDir()
	fp := filepath.Join(dir, "plain.lua")

	if err := os.WriteFile(fp, []byte("print(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, isTeal, err := resolveScript([]string{fp}, "", true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if isTeal {
		t.Errorf("isTeal true for .lua file with --teal; extension should win")
	}
}

func TestResolveScript_conflictBetweenFileAndCode(t *testing.T) {
	_, _, _, err := resolveScript([]string{"foo.lua"}, "print(1)", false)
	if err == nil {
		t.Fatalf("expected error when both file and --code are supplied")
	}
}

func TestResolveScript_noSourceIsError(t *testing.T) {
	_, _, _, err := resolveScript(nil, "", false)
	if err == nil {
		t.Fatalf("expected error with no source")
	}
}
