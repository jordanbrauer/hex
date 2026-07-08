package provider

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	hexlog "github.com/jordanbrauer/hex/log"
)

// ServeCommand returns a cobra `serve` command that runs the app as an
// HTTP server in the foreground. Consumer apps mount it under their
// root command; the scaffolder wires it automatically when `hex init
// --web` is used.
//
// The web provider registers *web.Server during hex.App.Bootstrap and
// starts the listener in its Boot phase, so by the time this command's
// RunE fires the server is already accepting connections. RunE
// therefore just blocks until SIGINT or SIGTERM, letting main.go's
// deferred Shutdown drain the server gracefully.
func ServeCommand(app *hex.App) *cobra.Command {
	_ = app // reserved for future use (metrics, container access, etc.)

	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP server in the foreground",
		Long: "Blocks until SIGINT or SIGTERM is received, then hands off\n" +
			"to the deferred app shutdown for graceful cleanup.\n\n" +
			"The web listener is started during app bootstrap (see the\n" +
			"web provider's Boot); this command's only job is to keep\n" +
			"the process alive.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			hexlog.Info("web: serving (press Ctrl+C to stop)")
			<-ctx.Done()
			hexlog.Info("web: shutdown signalled")

			return nil
		},
	}
}
