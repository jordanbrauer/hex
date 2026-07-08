package cli

import (
	"context"

	"github.com/jordanbrauer/hex"
)

// ctxKey is an unexported type for context keys to avoid collisions with
// keys other packages might use.
type ctxKey int

const appKey ctxKey = 0

// WithApp returns a new context carrying app.
func WithApp(ctx context.Context, app *hex.App) context.Context {
	return context.WithValue(ctx, appKey, app)
}

// FromContext extracts the *hex.App stored in ctx, or nil if none is present.
// Subcommands typically read it with:
//
//	app := hexcli.FromContext(cmd.Context())
func FromContext(ctx context.Context) *hex.App {
	if ctx == nil {
		return nil
	}

	app, _ := ctx.Value(appKey).(*hex.App)

	return app
}
