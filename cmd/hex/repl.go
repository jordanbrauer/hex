package main

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex/lua/repl"
)

func newReplCommand() *cobra.Command {
	var modeFlag string

	cmd := &cobra.Command{
		Use:   "repl",
		Short: "Interactive Teal / Lua / Fennel REPL",
		Long: `Launch an interactive REPL that evaluates Teal by default.
Use --mode to start in a different language:

  hex repl              # Teal (default)
  hex repl --mode lua   # Lua
  hex repl --mode fnl   # Fennel

In interactive mode, switch languages on the fly at an empty prompt:

  t   → Teal    (color: #3e8b9b)
  l   → Lua     (color: #000080)
  f   → Fennel  (color: #63b132)

Backspace on an empty prompt in a non-default mode returns to the
language you launched with.

The runtime is bare gopher-lua + the requested compiler; no hex
modules are pre-loaded. Scaffolded apps get a container-aware REPL
via "<appname> repl" — that one has access to db, cache, config,
and whatever the app registers.

Exit with Ctrl+D, "exit", or "quit".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := parseReplMode(modeFlag)
			if err != nil {
				return err
			}

			return repl.Run(repl.Options{
				Mode:        mode,
				In:          cmd.InOrStdin(),
				Out:         cmd.OutOrStdout(),
				ErrOut:      cmd.ErrOrStderr(),
				AppName:     "hex",
				Interactive: isatty.IsTerminal(os.Stdin.Fd()),
			})
		},
	}

	cmd.Flags().StringVar(&modeFlag, "mode", "", "starting language: teal (default), lua, fennel")

	return cmd
}

// parseReplMode maps the --mode flag string to a repl.Mode. Empty
// string defaults to Teal.
func parseReplMode(s string) (repl.Mode, error) {
	switch s {
	case "", "teal", "tl":
		return repl.ModeTeal, nil
	case "lua":
		return repl.ModeLua, nil
	case "fennel", "fnl":
		return repl.ModeFennel, nil
	default:
		return repl.ModeTeal, fmt.Errorf("repl: unknown --mode %q (expected teal, lua, or fennel)", s)
	}
}
