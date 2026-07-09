package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	hexlua "github.com/jordanbrauer/hex/lua"
)

func newRunCommand() *cobra.Command {
	var (
		code     string
		check    bool
		langFlag string
	)

	cmd := &cobra.Command{
		Use:   "run [file]",
		Short: "Run a Lua, Teal, or Fennel script",
		Long: `Run an arbitrary Lua (.lua), Teal (.tl), or Fennel (.fnl) script
or inline code.

Source can come from three places (mutually exclusive):

  hex run script.lua                  # a file (extension picks the language)
  hex run script.tl
  hex run script.fnl
  hex run -                           # stdin (--lang selects; default lua)
  hex run -c 'print("hi")'            # inline Lua source
  hex run -c '(print "hi")' --lang fnl # inline Fennel

Use --check to validate without executing. For .tl files this runs
the Teal type-checker; for .fnl files this runs the Fennel
compiler as a syntax check; for .lua files, the Lua parser.

The runtime is bare gopher-lua + the requested compiler; no hex
modules (agent, db, etc.) are pre-registered. For app-scoped script
execution with access to registered modules, use the "repl"
command on your scaffolded app or add a subcommand there.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			forced, err := parseLangFlag(langFlag)
			if err != nil {
				return err
			}

			source, name, lang, err := resolveScript(args, code, forced)
			if err != nil {
				return err
			}

			env := hexlua.New()
			defer env.Close()

			if check {
				return env.CheckString(source, name, lang)
			}

			script, err := env.LoadString(source, name, lang)
			if err != nil {
				return err
			}

			return env.Exec(script)
		},
	}

	cmd.Flags().StringVarP(&code, "code", "c", "", "inline source code (mutually exclusive with a file arg)")
	cmd.Flags().BoolVar(&check, "check", false, "validate syntax/types without executing")
	cmd.Flags().StringVar(&langFlag, "lang", "", "force language for inline/stdin source: lua, teal, fennel (irrelevant for file args)")

	return cmd
}

// parseLangFlag converts the --lang string into a Language. Empty
// string is treated as "no forced language" (caller falls through
// to Lua for stdin / inline, or file extension for files).
func parseLangFlag(s string) (hexlua.Language, error) {
	switch s {
	case "":
		return hexlua.Lua, nil
	case "lua":
		return hexlua.Lua, nil
	case "teal", "tl":
		return hexlua.Teal, nil
	case "fennel", "fnl":
		return hexlua.Fennel, nil
	default:
		return hexlua.Lua, fmt.Errorf("run: unknown --lang %q (expected lua, teal, or fennel)", s)
	}
}

// resolveScript resolves script source + a synthetic filename + a
// Language from the CLI inputs. Precedence: --code beats a file
// arg. `-` as the file arg means stdin. File extensions determine
// the language for file args; forced overrides everything else for
// inline/stdin.
func resolveScript(args []string, code string, forced hexlua.Language) (string, string, hexlua.Language, error) {
	switch {
	case code != "" && len(args) > 0:
		return "", "", hexlua.Lua, errors.New("run: cannot use --code together with a file argument")

	case code != "":
		return code, "<inline>", forced, nil

	case len(args) == 0:
		return "", "", hexlua.Lua, errors.New("run: provide a file, use --code, or pipe source via stdin (`hex run -`)")

	case args[0] == "-":
		data, err := io.ReadAll(os.Stdin)
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

		// Extension wins over --lang for file args; --lang only
		// affects inline / stdin.
		return string(data), abs, hexlua.LanguageFor(path), nil
	}
}
