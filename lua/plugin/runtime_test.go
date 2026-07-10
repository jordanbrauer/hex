package plugin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	glua "github.com/yuin/gopher-lua"

	"github.com/jordanbrauer/hex/lua/plugin"
)

// pingModule backs require("extra") in TestNewRuntimeExecutor_withModuleAddsExtraBinding.
func pingModule(l *glua.LState) int {
	t := l.NewTable()
	l.SetFuncs(t, map[string]glua.LGFunction{
		"ping": func(l *glua.LState) int {
			l.Push(glua.LString("pong"))

			return 1
		},
	})
	l.Push(t)

	return 1
}

func TestNewRuntimeExecutor_wiresArgvCmdAndDisk(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "run.lua")
	out := filepath.Join(dir, "out.txt")

	if err := os.WriteFile(script, []byte(`
local disk = require("disk")
local parts = {}
for i = 1, argc do
  table.insert(parts, argv[i])
end
disk.write(cmd.flags().get_string("out"), cmd.name .. ":" .. table.concat(parts, ","))
`), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	cmd := &cobra.Command{Use: "hello"}
	cmd.Flags().String("out", "", "output path")

	if err := cmd.Flags().Set("out", out); err != nil {
		t.Fatalf("set --out: %v", err)
	}

	exec := plugin.NewRuntimeExecutor()

	if err := exec(script, cmd, []string{"alice", "bob"}); err != nil {
		t.Fatalf("exec: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if want := "hello:alice,bob"; string(got) != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestNewRuntimeExecutor_withModuleAddsExtraBinding(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "run.lua")

	if err := os.WriteFile(script, []byte(`
local extra = require("extra")
assert(extra.ping() == "pong")
`), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	exec := plugin.NewRuntimeExecutor(plugin.WithModule("extra", pingModule))

	if err := exec(script, &cobra.Command{Use: "hello"}, nil); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func TestNewRuntimeExecutor_scriptErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "run.lua")

	if err := os.WriteFile(script, []byte(`error("boom")`), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	exec := plugin.NewRuntimeExecutor()

	if err := exec(script, &cobra.Command{Use: "hello"}, nil); err == nil {
		t.Fatal("expected error from failing script")
	}
}
