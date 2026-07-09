package main

import (
	"embed"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// helpFS holds the long-form description (`<key>.long.md`) and example
// script (`<key>.example.sh`) for every command, keyed by the command's
// snake-case identifier (matching its Go source file). Keeping help text
// in embedded files — rather than inline Go string literals — keeps it
// diff-friendly and lets `hex gen-man` reuse the exact same prose for the
// generated manpage.
//
//go:embed all:help
var helpFS embed.FS

// helpLong returns the embedded long description for a command key.
func helpLong(key string) string {
	return mustHelp(key + ".long.md")
}

// helpExample returns the embedded example script for a command key.
func helpExample(key string) string {
	return mustHelp(key + ".example.sh")
}

// setExample sets cmd.Example from the embedded example script for key.
func setExample(cmd *cobra.Command, key string) {
	cmd.Example = helpExample(key)
}

// mustHelp reads an embedded help file. The content is compiled into the
// binary, so a missing file is a build-time bug and panics immediately
// (surfaced the first time any command is constructed).
func mustHelp(name string) string {
	data, err := helpFS.ReadFile("help/" + name)
	if err != nil {
		panic(fmt.Sprintf("missing embedded help file %q: %v", name, err))
	}

	return strings.TrimRight(string(data), "\n")
}
