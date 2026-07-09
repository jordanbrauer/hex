package provider

import (
	"github.com/jordanbrauer/hex/lua/provider"
)

// Lua wires a shared *hex/lua.Environment into the container as "lua".
// Every provider that wants to expose itself to the REPL (or to scripts)
// resolves this in its own Register and calls env.PreloadModule /
// env.SetGlobal on it.
//
// The REPL (`swapi repl`) evaluates against this same
// environment, so anything you register here shows up interactively.
// See app/provider/repl_bindings.go for the pattern.
func Lua() *provider.Provider {
	return &provider.Provider{}
}
