// Package provider holds the hex CLI's own service providers — the same
// role app/provider plays in any scaffolded hex app.
package provider

import (
	"github.com/jordanbrauer/hex/lua/provider"
)

// Lua wires a shared *hex/lua.Environment into the container as "lua".
// It backs the `hex run` / `hex repl` commands (see app/command/root.go),
// which resolve the environment from the container instead of building
// their own — the same pattern any scaffolded hex app uses.
func Lua() *provider.Provider {
	return &provider.Provider{}
}
