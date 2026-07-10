// Command hex is the scaffolding CLI for hex applications.
//
// Usage:
//
//	hex init [name]              # scaffold a new project
//	hex make:provider <name>     # generate a service provider
//	hex make:domain <name>       # generate a domain package
//	hex make:migration <name>    # generate up/down migration files
//
// Run without arguments to see the full command list.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex/build"
	"github.com/jordanbrauer/hex/lua/plugin"
)

// commandPluginDir is where hex looks for repo-local command plugins
// (Lua/Teal/Fennel), relative to the current working directory. A
// missing directory is not an error — see plugin.LoadInto.
const commandPluginDir = ".hex/command"

// commandPluginGroup groups repo-local plugin commands under their
// own heading in `hex --help`, separate from hex's built-in commands.
var commandPluginGroup = plugin.Group{ID: "plugins", Title: "Plugins:"}

func main() {
	root := newRoot()

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hex",
		Short: "Scaffolding CLI for hex applications",
		Long: "hex is the scaffolding CLI for the hex Go framework.\n" +
			"Create new projects with `hex init` and add code with `hex make:*`.",
		Version:       hexVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.CompletionOptions.HiddenDefaultCmd = true

	cmd.AddCommand(
		newInitCommand(),
		newPublishCommand(),
		newRunCommand(),
		newReplCommand(),
		newMakeProviderCommand(),
		newMakeDomainCommand(),
		newMakeMigrationCommand(),
		newMakeCommandCommand(),
		newMakeAdapterCommand(),
		newMakeControllerCommand(),
		newGenManCommand(),
	)

	loadCommandPlugins(cmd)

	return cmd
}

// loadCommandPlugins mounts repo-local plugins found under
// commandPluginDir (".hex/command", relative to cwd) onto cmd. A
// missing directory is a silent no-op. A malformed plugin only warns
// — it must not prevent the rest of hex's commands from working.
func loadCommandPlugins(cmd *cobra.Command) {
	exec := plugin.NewRuntimeExecutor()

	if err := plugin.LoadInto(cmd, commandPluginGroup, []string{commandPluginDir}, exec); err != nil {
		fmt.Fprintln(os.Stderr, "warning: loading", commandPluginDir, "plugins:", err)
	}
}

func hexVersion() string {
	v := build.Version()
	if v == build.UnknownVersion {
		return "dev (" + build.ShortCommit() + ")"
	}

	return v
}
