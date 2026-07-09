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
		code   string
		check  bool
		asTeal bool
	)

	cmd := &cobra.Command{
		Use:   "run [file]",
		Short: "Run a Lua or Teal script",
		Long: `Run an arbitrary Lua (.lua) or Teal (.tl) script or inline code.

Source can come from three places (mutually exclusive):

  hex run script.lua             # a file (extension picks Lua/Teal)
  hex run script.tl
  hex run -                      # stdin (defaults to Lua; --teal to switch)
  hex run -c 'print("hi")'       # inline Lua source
  hex run -c '...' --teal        # inline Teal source

Use --check to validate without executing. For .tl files, this runs
the Teal type-checker; for .lua files, the Lua parser. Errors are
printed to stderr with source locations.

The runtime is bare gopher-lua + Teal compiler; no hex modules
(agent, http, etc.) are pre-registered. For app-scoped script
execution with access to registered modules, add a subcommand to
your scaffolded app that resolves the container's Lua environment.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source, name, isTeal, err := resolveScript(args, code, asTeal)
			if err != nil {
				return err
			}

			env := hexlua.New()
			defer env.Close()

			if check {
				return env.CheckString(source, name, isTeal)
			}

			script, err := env.LoadString(source, name, isTeal)
			if err != nil {
				return err
			}

			return env.Exec(script)
		},
	}

	cmd.Flags().StringVarP(&code, "code", "c", "", "inline source code (mutually exclusive with a file arg)")
	cmd.Flags().BoolVar(&check, "check", false, "validate syntax/types without executing")
	cmd.Flags().BoolVar(&asTeal, "teal", false, "treat inline/stdin source as Teal (default: Lua; irrelevant for file args)")

	return cmd
}

// resolveScript resolves script source + a synthetic filename + a
// isTeal flag from the CLI inputs. Precedence: --code beats a file
// arg. `-` as the file arg means stdin.
func resolveScript(args []string, code string, forceTeal bool) (string, string, bool, error) {
	switch {
	case code != "" && len(args) > 0:
		return "", "", false, errors.New("run: cannot use --code together with a file argument")

	case code != "":
		return code, "<inline>", forceTeal, nil

	case len(args) == 0:
		return "", "", false, errors.New("run: provide a file, use --code, or pipe source via stdin (`hex run -`)")

	case args[0] == "-":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", "", false, fmt.Errorf("run: read stdin: %w", err)
		}

		return string(data), "<stdin>", forceTeal, nil

	default:
		path := args[0]

		data, err := os.ReadFile(path)
		if err != nil {
			return "", "", false, fmt.Errorf("run: read %s: %w", path, err)
		}

		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			abs = path
		}

		// Extension wins over --teal for file args; --teal is only for
		// inline / stdin where there's no extension to consult.
		isTeal := filepath.Ext(path) == ".tl"

		return string(data), abs, isTeal, nil
	}
}
