package plugin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex/lua/plugin"
)

// writePlugin creates dir/config.toml plus any named files (contents
// keyed by filename) and returns dir.
func writePlugin(t *testing.T, dir, configTOML string, files map[string]string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	if configTOML != "" {
		if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(configTOML), 0o644); err != nil {
			t.Fatalf("write config.toml: %v", err)
		}
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	return dir
}

func noopExecutor(_ string, _ *cobra.Command, _ []string) error { return nil }

func TestNewPlugin_missingConfig(t *testing.T) {
	dir := t.TempDir()

	if _, err := plugin.NewPlugin(dir); err == nil {
		t.Fatal("expected error for missing config.toml")
	}
}

func TestNewPlugin_missingUse(t *testing.T) {
	dir := writePlugin(t, t.TempDir(), `short = "no use field"`, nil)

	if _, err := plugin.NewPlugin(dir); err == nil {
		t.Fatal("expected error for missing `use`")
	}
}

func TestNewPlugin_ok(t *testing.T) {
	dir := writePlugin(t, t.TempDir(), `
use = "hello"
aliases = ["hi"]
short = "say hello"
`, map[string]string{"run.lua": `print("hi")`})

	p, err := plugin.NewPlugin(dir)
	if err != nil {
		t.Fatalf("NewPlugin: %v", err)
	}

	if p.Use != "hello" {
		t.Errorf("Use = %q, want hello", p.Use)
	}

	if len(p.Aliases) != 1 || p.Aliases[0] != "hi" {
		t.Errorf("Aliases = %v, want [hi]", p.Aliases)
	}
}

func TestPlugin_Command_missingEntrypoint(t *testing.T) {
	dir := writePlugin(t, t.TempDir(), `use = "empty"`, nil)

	p, err := plugin.NewPlugin(dir)
	if err != nil {
		t.Fatalf("NewPlugin: %v", err)
	}

	if _, err := p.Command(noopExecutor); err == nil {
		t.Fatal("expected error for plugin with no run.* and no commands")
	}
}

func TestPlugin_Command_ambiguousEntrypoint(t *testing.T) {
	dir := writePlugin(t, t.TempDir(), `use = "ambiguous"`, map[string]string{
		"run.lua": `print("lua")`,
		"run.tl":  `print("tl")`,
	})

	p, err := plugin.NewPlugin(dir)
	if err != nil {
		t.Fatalf("NewPlugin: %v", err)
	}

	_, err = p.Command(noopExecutor)
	if err == nil {
		t.Fatal("expected error for ambiguous entrypoint")
	}

	if !strings.Contains(err.Error(), "ambiguous entrypoint") {
		t.Errorf("error = %v, want mentions ambiguous entrypoint", err)
	}
}

func TestPlugin_Command_wiresRunAndArgs(t *testing.T) {
	dir := writePlugin(t, t.TempDir(), `
use   = "hello"
short = "say hello"

[flags]
name = { type = "string", usage = "who to greet", value = "world" }
`, map[string]string{
		"run.lua":  `print("run")`,
		"args.lua": `print("args")`,
	})

	p, err := plugin.NewPlugin(dir)
	if err != nil {
		t.Fatalf("NewPlugin: %v", err)
	}

	var gotRun, gotArgs string

	exec := func(path string, _ *cobra.Command, _ []string) error {
		switch filepath.Base(path) {
		case "run.lua":
			gotRun = path
		case "args.lua":
			gotArgs = path
		}

		return nil
	}

	cmd, err := p.Command(exec)
	if err != nil {
		t.Fatalf("Command: %v", err)
	}

	if cmd.Use != "hello" || cmd.Short != "say hello" {
		t.Errorf("cmd = %+v, want Use=hello Short=%q", cmd, "say hello")
	}

	if cmd.Flags().Lookup("name") == nil {
		t.Error("expected `name` flag to be registered")
	}

	if err := cmd.Args(cmd, nil); err != nil {
		t.Fatalf("Args: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	if gotRun == "" {
		t.Error("RunE did not call exec with run.lua")
	}

	if gotArgs == "" {
		t.Error("Args did not call exec with args.lua")
	}
}

func TestPlugin_Command_recursesChildren(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, `
use      = "job"
short    = "job commands"
commands = ["list"]
`, nil)
	writePlugin(t, filepath.Join(root, "list"), `
use   = "list"
short = "list jobs"
`, map[string]string{"run.lua": `print("list")`})

	p, err := plugin.NewPlugin(root)
	if err != nil {
		t.Fatalf("NewPlugin: %v", err)
	}

	cmd, err := p.Command(noopExecutor)
	if err != nil {
		t.Fatalf("Command: %v", err)
	}

	child, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("Find(list): %v", err)
	}

	if child.Use != "list" {
		t.Errorf("child.Use = %q, want list", child.Use)
	}
}

func TestPlugin_Command_missingChildDir(t *testing.T) {
	dir := writePlugin(t, t.TempDir(), `
use      = "job"
commands = ["missing"]
`, nil)

	p, err := plugin.NewPlugin(dir)
	if err != nil {
		t.Fatalf("NewPlugin: %v", err)
	}

	if _, err := p.Command(noopExecutor); err == nil {
		t.Fatal("expected error for missing child command dir")
	}
}
