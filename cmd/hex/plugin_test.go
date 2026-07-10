package main

import (
	"os"
	"path/filepath"
	"testing"
)

// chdir switches the process into dir for the duration of the test,
// restoring the original working directory on cleanup. Command-plugin
// discovery is relative to cwd, so exercising it end-to-end means
// running from a directory that actually has a .hex/command tree.
func chdir(t *testing.T, dir string) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func TestNewRoot_loadsCommandPlugins(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, ".hex", "command", "hello")

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	config := `
use   = "hello"
short = "say hello"

[flags]
out = { type = "string", usage = "output path", value = "" }
`
	if err := os.WriteFile(filepath.Join(pluginDir, "config.toml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	script := `
local disk = require("disk")
disk.write(cmd.flags().get_string("out"), "hello from plugin")
`
	if err := os.WriteFile(filepath.Join(pluginDir, "run.lua"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}

	chdir(t, dir)

	out := filepath.Join(dir, "out.txt")

	root := newRoot()
	root.SetArgs([]string{"hello", "--out", out})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if want := "hello from plugin"; string(got) != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestNewRoot_noHexDirIsNoop(t *testing.T) {
	chdir(t, t.TempDir())

	root := newRoot()

	for _, g := range root.Groups() {
		if g.ID == commandPluginGroup.ID {
			t.Error("plugins group present despite no .hex/command dir")
		}
	}
}
