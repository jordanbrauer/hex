// Package command holds the hex CLI's own cobra command tree — built the
// same way `hex init` wires one for a scaffolded app.
package command

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	hexbuild "github.com/jordanbrauer/hex/build"
	"github.com/jordanbrauer/hex/cli"
	"github.com/jordanbrauer/hex/lua/plugin"
	luaprovider "github.com/jordanbrauer/hex/lua/provider"

	"github.com/jordanbrauer/hex/cmd/hex/app/build"
	"github.com/jordanbrauer/hex/cmd/hex/app/command/genman"
	initcmd "github.com/jordanbrauer/hex/cmd/hex/app/command/init"
	makegroup "github.com/jordanbrauer/hex/cmd/hex/app/command/make"
	"github.com/jordanbrauer/hex/cmd/hex/app/command/publish"
)

// commandPluginDir is where hex looks for repo-local command plugins
// (Lua/Teal/Fennel), relative to the current working directory. A
// missing directory is not an error — see plugin.LoadInto.
const commandPluginDir = ".hex/command"

// commandPluginGroup groups repo-local plugin commands under their own
// heading in `hex --help`, separate from hex's built-in commands.
var commandPluginGroup = plugin.Group{ID: "plugins", Title: "Plugins:"}

// Execute builds the command tree wired to app and runs it, returning the
// process exit code. main is just os.Exit(command.Execute(kernel)).
func Execute(app *hex.App) int {
	return cli.Execute(Root(app))
}

// Root builds the top-level cobra command wired to app. hex make command
// inserts subcommand registrations above the `// hex:commands` marker
// below. Do not remove the marker.
func Root(app *hex.App) *cobra.Command {
	root := cli.Root(cli.RootOptions{
		Name:  "hex",
		Short: "Scaffolding CLI for hex applications",
		Long: "hex is the scaffolding CLI for the hex Go framework.\n" +
			"Create new projects with `hex init` and add code with `hex make`.",
		App: app,
	})
	root.Version = hexVersion()
	root.CompletionOptions.HiddenDefaultCmd = true

	// Framework-provided commands. Each framework provider that exposes
	// a subcommand does so through its provider package.
	root.AddCommand(luaprovider.ReplCommand(app))
	root.AddCommand(luaprovider.RunCommand(app))

	root.AddCommand(
		initcmd.New(app),
		publish.New(app),
		makegroup.New(app),
		// hex:commands
	)

	// gen-man introspects the tree above it was just given, so it's
	// added last — by the time its RunE runs, root already has every
	// other command (including gen-man itself) attached.
	root.AddCommand(genman.New(root))

	loadCommandPlugins(root)

	return root
}

// loadCommandPlugins mounts repo-local plugins found under
// commandPluginDir (".hex/command", relative to cwd) onto cmd. A missing
// directory is a silent no-op. A malformed plugin only warns — it must
// not prevent the rest of hex's commands from working.
func loadCommandPlugins(cmd *cobra.Command) {
	exec := plugin.NewRuntimeExecutor()

	if err := plugin.LoadInto(cmd, commandPluginGroup, []string{commandPluginDir}, exec); err != nil {
		fmt.Fprintln(os.Stderr, "warning: loading", commandPluginDir, "plugins:", err)
	}
}

// hexVersion returns build.Info().Version, or a "dev (<short-commit>)"
// fallback when no version was injected via ldflags.
func hexVersion() string {
	info := build.Info()
	if info.Version == hexbuild.UnknownVersion {
		return "dev (" + hexbuild.ShortCommit() + ")"
	}

	return info.Version
}
