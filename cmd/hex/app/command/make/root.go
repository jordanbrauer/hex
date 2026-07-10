// Package make holds the `hex make` command group — one subpackage per
// generator (provider, domain, migration, adapter, controller, command).
package make

import (
	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"

	"github.com/jordanbrauer/hex/cmd/hex/app/command/make/adapter"
	makecommand "github.com/jordanbrauer/hex/cmd/hex/app/command/make/command"
	"github.com/jordanbrauer/hex/cmd/hex/app/command/make/controller"
	"github.com/jordanbrauer/hex/cmd/hex/app/command/make/domain"
	"github.com/jordanbrauer/hex/cmd/hex/app/command/make/migration"
	"github.com/jordanbrauer/hex/cmd/hex/app/command/make/provider"
)

// New builds the `hex make` command group.
func New(app *hex.App) *cobra.Command {
	cmd := new(cobra.Command)

	cmd.Use = "make"
	cmd.Short = "Generate correctly-placed, correctly-wired code for a hex project"

	cmd.AddCommand(
		provider.New(app),
		domain.New(app),
		migration.New(app),
		adapter.New(app),
		controller.New(app),
		makecommand.New(app),
	)

	return cmd
}
