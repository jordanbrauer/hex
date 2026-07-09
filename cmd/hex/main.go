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
)

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
	)

	return cmd
}

func hexVersion() string {
	v := build.Version()
	if v == build.UnknownVersion {
		return "dev (" + build.ShortCommit() + ")"
	}

	return v
}
