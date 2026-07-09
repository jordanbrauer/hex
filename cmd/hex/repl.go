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
		Long:  helpLong("repl"),
		Args:  cobra.NoArgs,
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
	setExample(cmd, "repl")

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
