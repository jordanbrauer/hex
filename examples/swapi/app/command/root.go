// Package command holds this application's cobra command tree.
package command

import (
	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	hexcli "github.com/jordanbrauer/hex/cli"
	luaprovider "github.com/jordanbrauer/hex/lua/provider"
	webprovider "github.com/jordanbrauer/hex/web/provider"

	"github.com/jordanbrauer/hex/examples/swapi/app/build"
)

// Root builds the top-level cobra command wired to app. hex make:command
// inserts subcommand registrations above the `// hex:commands` marker
// below. Do not remove the marker.
func Root(app *hex.App) *cobra.Command {
	root := hexcli.Root(hexcli.RootOptions{
		Name:  "swapi",
		Short: "swapi",
		App:   app,
	})

	root.AddCommand(hexcli.Version(hexcli.VersionOptions{
		App: build.Info().Name,
	}))

	// Framework-provided commands. Each framework provider that
	// exposes a subcommand does so through its provider package.
	root.AddCommand(luaprovider.ReplCommand(app))
	root.AddCommand(luaprovider.RunCommand(app))
	root.AddCommand(webprovider.ServeCommand(app))

	root.AddCommand(
	// hex:commands
	)

	return root
}
