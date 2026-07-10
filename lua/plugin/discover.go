package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Group names the cobra.Group discovered commands are tagged with, so
// they show under their own heading in --help.
type Group struct {
	ID    string
	Title string
}

// Discover reads dir's immediate subdirectories and loads each one
// that contains a config.toml as a Plugin. Non-plugin subdirectories
// are skipped. A missing dir is not an error — it returns (nil, nil)
// so callers can treat "no plugin directory" as a no-op.
func Discover(dir string) ([]*Plugin, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("plugin: read %s: %w", dir, err)
	}

	var plugins []*Plugin

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		sub := filepath.Join(dir, e.Name())

		if _, err := os.Stat(filepath.Join(sub, "config.toml")); err != nil {
			continue
		}

		p, err := NewPlugin(sub)
		if err != nil {
			return nil, err
		}

		plugins = append(plugins, p)
	}

	return plugins, nil
}

// LoadInto discovers plugins under each of dirs (in order) and adds
// them to root as subcommands tagged with group. A command name
// already present on root — whether a built-in command or a plugin
// loaded from an earlier dir — wins; later duplicates are silently
// skipped. exec runs every discovered plugin's entrypoints; see
// NewRuntimeExecutor.
func LoadInto(root *cobra.Command, group Group, dirs []string, exec Executor) error {
	seen := make(map[string]struct{}, len(root.Commands()))
	for _, existing := range root.Commands() {
		seen[existing.Name()] = struct{}{}
	}

	groupAdded := false

	for _, dir := range dirs {
		plugins, err := Discover(dir)
		if err != nil {
			return err
		}

		for _, p := range plugins {
			cmd, err := p.Command(exec)
			if err != nil {
				return fmt.Errorf("plugin %s: %w", p.dir, err)
			}

			if _, exists := seen[cmd.Name()]; exists {
				continue
			}

			seen[cmd.Name()] = struct{}{}

			if !groupAdded {
				root.AddGroup(&cobra.Group{ID: group.ID, Title: group.Title})
				groupAdded = true
			}

			cmd.GroupID = group.ID
			root.AddCommand(cmd)
		}
	}

	return nil
}
