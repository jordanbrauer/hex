package provider

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/config"
	"github.com/jordanbrauer/hex/container"
	"github.com/jordanbrauer/hex/env"
	hexlua "github.com/jordanbrauer/hex/lua"
	"github.com/jordanbrauer/hex/lua/repl"
)

// ReplCommand returns a cobra `repl` command that opens an interactive
// Teal/Lua REPL against the app's shared *hex/lua.Environment.
//
// The REPL sees every Lua module registered by framework providers
// (agent, and — as Phase 3 lands — db, config, cache, log, events,
// queue, env) and by consumer providers (domain services). This is
// the Tinker / Rails console / Phoenix IEx analogue for hex apps:
// shell into a pod, inspect the live DB, prototype a script.
//
// Consumer apps mount it under their root command; the scaffolder
// wires it automatically. Wire it manually with:
//
//	rootCmd.AddCommand(luaprovider.ReplCommand(app))
//
// Mode selection precedence (highest to lowest):
//  1. --mode flag on the command
//  2. repl.mode in the app config
//  3. Teal (framework default)
//
// A prod banner is added when app.Environment() reports Production.
func ReplCommand(app *hex.App) *cobra.Command {
	var overrideMode string

	cmd := &cobra.Command{
		Use:   "repl",
		Short: "Interactive REPL wired to the app container",
		Long: "Opens an interactive Teal (default) or Lua REPL against\n" +
			"the app's shared Lua environment. Every module registered\n" +
			"by framework and consumer providers is available via\n" +
			"require(). Use `global x: T = v` in Teal mode to declare\n" +
			"variables that persist across lines.\n\n" +
			"Exit with Ctrl+D, \"exit\", or \"quit\".",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			luaEnv, err := container.Make[*hexlua.Environment](app.Container(), "lua")
			if err != nil {
				return fmt.Errorf("repl: resolve lua environment: %w", err)
			}

			mode := resolveMode(app, overrideMode)
			banner := prodBanner(app)
			appName := binaryName()

			return repl.Run(repl.Options{
				Mode:        mode,
				In:          cmd.InOrStdin(),
				Out:         cmd.OutOrStdout(),
				ErrOut:      cmd.ErrOrStderr(),
				AppName:     appName,
				Banner:      banner,
				Env:         luaEnv,
				Interactive: isatty.IsTerminal(os.Stdin.Fd()),
			})
		},
	}

	cmd.Flags().StringVar(&overrideMode, "mode", "", "override configured mode: teal | lua")

	return cmd
}

// resolveMode picks the REPL language: --mode flag first, then
// config `repl.mode`, then the Teal default.
func resolveMode(app *hex.App, override string) repl.Mode {
	if m := parseMode(override); m != nil {
		return *m
	}

	if store, err := container.Make[*config.Store](app.Container(), "config"); err == nil {
		if m := parseMode(store.String("repl.mode")); m != nil {
			return *m
		}
	}

	return repl.ModeTeal
}

// parseMode returns the parsed Mode or nil for empty / unknown input,
// letting callers fall through to the next source in precedence.
func parseMode(s string) *repl.Mode {
	switch s {
	case "teal", "tl":
		m := repl.ModeTeal

		return &m
	case "lua":
		m := repl.ModeLua

		return &m
	case "fennel", "fnl":
		m := repl.ModeFennel

		return &m
	default:
		return nil
	}
}

// prodBanner returns a warning line when the app is running in the
// Production environment, else the empty string.
func prodBanner(app *hex.App) string {
	if app.Environment() == env.Production {
		return "\u26a0  connected to PRODUCTION \u2014 writes are real."
	}

	return ""
}

// binaryName returns the executable's basename ("myapp", "hex") for
// use as the REPL prompt prefix. Falls back to "app" if the process
// argv is unavailable for some reason.
func binaryName() string {
	if len(os.Args) == 0 || os.Args[0] == "" {
		return "app"
	}

	name := filepath.Base(os.Args[0])
	if name == "" || name == "." || name == "/" {
		return "app"
	}

	return name
}
