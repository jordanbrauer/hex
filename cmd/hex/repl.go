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
	"github.com/jordanbrauer/hex/lua/teal"
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

	// For Teal mode: initialise a persistent Teal env so declarations
	// on one line remain visible on subsequent lines. Non-REPL
	// contexts (hex run script.tl) intentionally get isolated chunk
	// semantics; the REPL is the exception.
	var tealSession *teal.Session

	if isTeal {
		if err := teal.Load(env.L); err != nil {
			return fmt.Errorf("repl: load teal: %w", err)
		}

		session, err := teal.NewSession(env.L)
		if err != nil {
			return fmt.Errorf("repl: teal session: %w", err)
		}

		tealSession = session
	}

	fmt.Fprintf(out, "hex repl — %s mode. Ctrl+D or \"exit\" to quit.\n", mode)

	if isTeal {
		fmt.Fprintln(out, "note: Teal forbids implicit globals. Use `global x: T = v` to declare persistent variables;")
		fmt.Fprintln(out, "      locals do not carry across REPL lines. Use `--lua` for looser Lua semantics.")
	}

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

		if err := evalLine(env, out, line, isTeal, tealSession); err != nil {
			msg := trimTraceback(err.Error())
			fmt.Fprintln(errOut, "error:", msg)

			// Teal-specific hint for the most common REPL confusion:
			// `foo = 12` in Teal errors as "unknown variable: foo".
			// Point the user at the fix.
			if isTeal && strings.Contains(msg, "unknown variable") {
				fmt.Fprintln(errOut, "hint: prefix with `global` to declare, e.g. `global foo: number = 12`")
			}
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
//
// When session is non-nil, Teal compilation uses the persistent env
// so declarations carry over across REPL lines.
func evalLine(env *hexlua.Environment, out io.Writer, line string, isTeal bool, session *teal.Session) error {
	// Expression form: prepend `return (` … `)` to capture the value.
	wrapped := "return (" + line + ")"

	if script, err := loadReplChunk(env, session, wrapped, isTeal); err == nil {
		return execAndPrint(env, out, script)
	}

	// Fall back to statement.
	script, err := loadReplChunk(env, session, line, isTeal)
	if err != nil {
		return err
	}

	return env.Exec(script)
}

// loadReplChunk compiles a REPL line. When a Teal session is present,
// uses its persistent env so prior declarations stay in scope.
func loadReplChunk(env *hexlua.Environment, session *teal.Session, source string, isTeal bool) (*hexlua.Script, error) {
	if !isTeal || session == nil {
		return env.LoadString(source, "<repl>", isTeal)
	}

	luaSrc, err := session.Compile(source, "<repl>")
	if err != nil {
		return nil, err
	}

	return hexlua.Compile(luaSrc, "<repl>")
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
