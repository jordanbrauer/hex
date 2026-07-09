package main

import (
	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex/lua/repl"
)

func newReplCommand() *cobra.Command {
	var asLua bool

	cmd := &cobra.Command{
		Use:   "repl",
		Short: "Interactive Teal (or Lua) REPL",
		Long: `Launch an interactive REPL that evaluates Teal by default.
Pass --lua for a Lua-only session.

Each line is first tried as an expression (wrapped in return (…)):
the value is printed if non-nil. If that fails to compile, the line
is executed as a statement.

The runtime is bare gopher-lua + Teal; no hex modules are pre-loaded.
Scaffolded apps get a container-aware REPL via their own binary
(e.g. "myapp repl") — that one has access to db, cache, config, and
whatever the app registers itself.

Exit with Ctrl+D, "exit", or "quit".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := repl.ModeTeal
			if asLua {
				mode = repl.ModeLua
			}

			return repl.Run(repl.Options{
				Mode:    mode,
				In:      cmd.InOrStdin(),
				Out:     cmd.OutOrStdout(),
				ErrOut:  cmd.ErrOrStderr(),
				AppName: "hex",
			})
		},
	}

	cmd.Flags().BoolVar(&asLua, "lua", false, "evaluate input as Lua instead of Teal")

	return cmd
}
