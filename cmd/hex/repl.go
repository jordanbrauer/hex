package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	lua "github.com/yuin/gopher-lua"

	hexlua "github.com/jordanbrauer/hex/lua"
)

func newReplCommand() *cobra.Command {
	var (
		asLua bool
	)

	cmd := &cobra.Command{
		Use:   "repl",
		Short: "Interactive Teal (or Lua) REPL",
		Long: `Launch an interactive REPL that evaluates Teal by default.
Pass --lua for a Lua-only session.

Each line is first tried as an expression (wrapped in return (…)):
the value is printed if non-nil. If that fails to compile, the line
is executed as a statement.

The runtime is bare gopher-lua + Teal; no hex modules are pre-loaded.
Exit with Ctrl+D, "exit", or "quit".`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			isTeal := !asLua

			return runRepl(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), isTeal)
		},
	}

	cmd.Flags().BoolVar(&asLua, "lua", false, "evaluate input as Lua instead of Teal")

	return cmd
}

// runRepl is the read-eval-print loop. Exported (unexported) as a
// function so tests can drive it with buffers instead of stdio.
func runRepl(in io.Reader, out, errOut io.Writer, isTeal bool) error {
	env := hexlua.New()
	defer env.Close()

	mode := "teal"
	if !isTeal {
		mode = "lua"
	}

	fmt.Fprintf(out, "hex repl — %s mode. Ctrl+D or \"exit\" to quit.\n", mode)

	prompt := "hex(" + mode + ")> "

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for {
		fmt.Fprint(out, prompt)

		if !scanner.Scan() {
			// EOF or read error. Distinguish by scanner.Err() == nil.
			if err := scanner.Err(); err != nil {
				return err
			}

			fmt.Fprintln(out)

			return nil
		}

		line := scanner.Text()

		if isExitDirective(line) {
			return nil
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		if err := evalLine(env, out, line, isTeal); err != nil {
			fmt.Fprintln(errOut, "error:", trimTraceback(err.Error()))
		}
	}
}

// isExitDirective recognises the shell-conventional "exit" / "quit"
// commands and their Lua-style ".exit" cousin (matches Lua 5.4's
// interactive interpreter).
func isExitDirective(line string) bool {
	switch strings.TrimSpace(line) {
	case "exit", "quit", ".exit", ".quit":
		return true
	default:
		return false
	}
}

// evalLine tries an expression form first (wrap in `return (...)`)
// and falls back to a statement form when the expression fails to
// compile. Print top-of-stack when the expression form succeeds and
// yielded a non-nil value.
func evalLine(env *hexlua.Environment, out io.Writer, line string, isTeal bool) error {
	// Expression form: prepend `return (` … `)` to capture the value.
	wrapped := "return (" + line + ")"

	if script, err := env.LoadString(wrapped, "<repl>", isTeal); err == nil {
		return execAndPrint(env, out, script)
	}

	// Fall back to statement.
	script, err := env.LoadString(line, "<repl>", isTeal)
	if err != nil {
		return err
	}

	return env.Exec(script)
}

// execAndPrint runs the compiled expression and prints its result if
// non-nil. Mirrors Lua's standard REPL behavior.
func execAndPrint(env *hexlua.Environment, out io.Writer, script *hexlua.Script) error {
	// The expression path expects exactly one return value on top of
	// the stack after the call.
	before := env.L.GetTop()

	if err := env.Exec(script); err != nil {
		return err
	}

	after := env.L.GetTop()

	// Some Lua chunks return zero values, some many. Print each in
	// order, one per line. Skip anything that's nil to avoid noise
	// from `return nil` expressions.
	for i := before + 1; i <= after; i++ {
		v := env.L.Get(i)
		if v == lua.LNil {
			continue
		}

		fmt.Fprintln(out, formatValue(v))
	}

	env.L.SetTop(before)

	return nil
}

// trimTraceback strips gopher-lua's default stack trace tail so REPL
// errors read as one-liners. The trace is still useful when
// scripting, but in the REPL every error would otherwise triple in
// height.
func trimTraceback(msg string) string {
	if i := strings.Index(msg, "\nstack traceback:"); i >= 0 {
		return strings.TrimSpace(msg[:i])
	}

	return strings.TrimSpace(msg)
}

// formatValue renders a Lua value for display in the REPL. Strings
// are shown unquoted (Ruby-style); everything else uses gopher-lua's
// default String() rendering.
func formatValue(v lua.LValue) string {
	if v == nil {
		return "nil"
	}

	if s, ok := v.(lua.LString); ok {
		return string(s)
	}

	return v.String()
}

// silence unused imports if the REPL is compiled without errors.
var (
	_ = errors.New
	_ = os.Stdin
)
