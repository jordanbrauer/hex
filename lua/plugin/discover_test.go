package plugin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex/lua/plugin"
)

func TestDiscover_missingDirIsNoop(t *testing.T) {
	plugins, err := plugin.Discover(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if plugins != nil {
		t.Errorf("plugins = %v, want nil", plugins)
	}
}

func TestDiscover_skipsNonPluginDirs(t *testing.T) {
	root := t.TempDir()

	// Not a plugin: no config.toml.
	if err := os.MkdirAll(filepath.Join(root, "not-a-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}

	writePlugin(t, filepath.Join(root, "hello"), `use = "hello"`, map[string]string{
		"run.lua": `print("hi")`,
	})

	plugins, err := plugin.Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(plugins) != 1 || plugins[0].Use != "hello" {
		t.Errorf("plugins = %+v, want exactly [hello]", plugins)
	}
}

func newTestRoot() *cobra.Command {
	return &cobra.Command{Use: "hex"}
}

func TestLoadInto_addsCommandsUnderGroup(t *testing.T) {
	root := newTestRoot()
	dir := t.TempDir()
	writePlugin(t, filepath.Join(dir, "hello"), `use = "hello"`, map[string]string{
		"run.lua": `print("hi")`,
	})

	group := plugin.Group{ID: "plugins", Title: "Plugins:"}

	if err := plugin.LoadInto(root, group, []string{dir}, noopExecutor); err != nil {
		t.Fatalf("LoadInto: %v", err)
	}

	cmd, _, err := root.Find([]string{"hello"})
	if err != nil {
		t.Fatalf("Find(hello): %v", err)
	}

	if cmd.GroupID != "plugins" {
		t.Errorf("GroupID = %q, want plugins", cmd.GroupID)
	}

	found := false
	for _, g := range root.Groups() {
		if g.ID == "plugins" {
			found = true
		}
	}

	if !found {
		t.Error("expected plugins group to be added to root")
	}
}

func TestLoadInto_existingRootCommandWins(t *testing.T) {
	root := newTestRoot()
	root.AddCommand(&cobra.Command{Use: "hello", Short: "built-in"})

	dir := t.TempDir()
	writePlugin(t, filepath.Join(dir, "hello"), `use = "hello"`, map[string]string{
		"run.lua": `print("hi")`,
	})

	group := plugin.Group{ID: "plugins", Title: "Plugins:"}

	if err := plugin.LoadInto(root, group, []string{dir}, noopExecutor); err != nil {
		t.Fatalf("LoadInto: %v", err)
	}

	cmd, _, err := root.Find([]string{"hello"})
	if err != nil {
		t.Fatalf("Find(hello): %v", err)
	}

	if cmd.Short != "built-in" {
		t.Errorf("Short = %q, want built-in (plugin must not clobber it)", cmd.Short)
	}
}

func TestLoadInto_firstDirWinsAcrossDirs(t *testing.T) {
	root := newTestRoot()

	dirA, dirB := t.TempDir(), t.TempDir()
	writePlugin(t, filepath.Join(dirA, "hello"), `
use   = "hello"
short = "from A"
`, map[string]string{"run.lua": `print("a")`})
	writePlugin(t, filepath.Join(dirB, "hello"), `
use   = "hello"
short = "from B"
`, map[string]string{"run.lua": `print("b")`})

	group := plugin.Group{ID: "plugins", Title: "Plugins:"}

	if err := plugin.LoadInto(root, group, []string{dirA, dirB}, noopExecutor); err != nil {
		t.Fatalf("LoadInto: %v", err)
	}

	cmd, _, err := root.Find([]string{"hello"})
	if err != nil {
		t.Fatalf("Find(hello): %v", err)
	}

	if cmd.Short != "from A" {
		t.Errorf("Short = %q, want from A (first dir wins)", cmd.Short)
	}
}
