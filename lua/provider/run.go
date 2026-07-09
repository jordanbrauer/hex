package provider

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jordanbrauer/hex"
	"github.com/jordanbrauer/hex/container"
	hexlua "github.com/jordanbrauer/hex/lua"
)

// RunCommand returns a cobra `run` command that executes a Lua, Teal,
// or Fennel script against the app's shared *hex/lua.Environment.
//
// This is the container-aware sibling of the top-level `hex run`
// command: same flags, same source-resolution rules — but the script
// sees every Lua module registered by framework providers (config,
// log, env, events, db, cache, queue, ai, …) and by consumer
// providers (domain services). It is the "throwaway one-liner
// against the live app" tool — Tinker's `--execute`, Rails's
// `runner`, Phoenix's `mix run -e`.
//
// Consumer apps mount it under their root command; the scaffolder
// wires it automatically. Wire it manually with:
//
//	rootCmd.AddCommand(luaprovider.RunCommand(app))
//
// Source resolution (same as `hex run`):
//
//	app run script.lua                  # file (extension picks language)
//	app run script.tl
//	app run script.fnl
//	app run -                           # stdin (--lang selects; default lua)
//	app run -c 'print(db.query(...))'   # inline
//	app run -c '(print (env.get "PORT"))' --lang fnl
//
// Use --check to validate without executing.
func RunCommand(app *hex.App) *cobra.Command {
	var (
		code     string
		check    bool
		langFlag string
	)

	cmd := &cobra.Command{
		Use:   "run [file]",
		Short: "Run a Lua/Teal/Fennel script against the app container",
		Long: "Executes a script inside the app's shared Lua environment,\n" +
			"so every module registered by framework and consumer\n" +
			"providers (db, config, log, env, events, cache, queue, …)\n" +
			"is directly available.\n\n" +
			"Source can come from a file arg, --code, or stdin (`-`).\n" +
			"File extensions (.lua, .tl, .fnl) pick the language;\n" +
			"--lang forces it for inline/stdin source.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			forced, err := parseRunLang(langFlag)
			if err != nil {
				return err
			}

			source, name, lang, err := resolveRunScript(args, code, forced, cmd.InOrStdin())
			if err != nil {
				return err
			}

			luaEnv, err := container.Make[*hexlua.Environment](app.Container(), "lua")
			if err != nil {
				return fmt.Errorf("run: resolve lua environment: %w", err)
			}

			if check {
				return luaEnv.CheckString(source, name, lang)
			}

			script, err := luaEnv.LoadString(source, name, lang)
			if err != nil {
				return err
			}

			return luaEnv.Exec(script)
		},
	}

	cmd.Flags().StringVarP(&code, "code", "c", "", "inline source (mutually exclusive with a file arg)")
	cmd.Flags().BoolVar(&check, "check", false, "validate syntax/types without executing")
	cmd.Flags().StringVar(&langFlag, "lang", "", "force language for inline/stdin source: lua, teal, fennel")

	return cmd
}

// parseRunLang mirrors cmd/hex/run.go's parseLangFlag. Kept here so
// the provider is a self-contained drop-in for scaffolded apps.
func parseRunLang(s string) (hexlua.Language, error) {
	switch s {
	case "", "lua":
		return hexlua.Lua, nil
	case "teal", "tl":
		return hexlua.Teal, nil
	case "fennel", "fnl":
		return hexlua.Fennel, nil
	default:
		return hexlua.Lua, fmt.Errorf("run: unknown --lang %q (expected lua, teal, or fennel)", s)
	}
}

// resolveRunScript is the container-aware twin of cmd/hex/run.go's
// resolveScript. It takes an explicit stdin reader so callers can
// pipe test data in during unit tests.
func resolveRunScript(args []string, code string, forced hexlua.Language, stdin io.Reader) (string, string, hexlua.Language, error) {
	switch {
	case code != "" && len(args) > 0:
		return "", "", hexlua.Lua, errors.New("run: cannot use --code together with a file argument")

	case code != "":
		return code, "<inline>", forced, nil

	case len(args) == 0:
		return "", "", hexlua.Lua, errors.New("run: provide a file, use --code, or pipe source via stdin (`run -`)")

	case args[0] == "-":
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", "", hexlua.Lua, fmt.Errorf("run: read stdin: %w", err)
		}

		return string(data), "<stdin>", forced, nil

	default:
		path := args[0]

		data, err := os.ReadFile(path)
		if err != nil {
			return "", "", hexlua.Lua, fmt.Errorf("run: read %s: %w", path, err)
		}

		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			abs = path
		}

		return string(data), abs, hexlua.LanguageFor(path), nil
	}
}
