// Package repl provides an interactive Read-Eval-Print Loop for
// Teal and Lua sources, wired to a caller-provided *lua.Environment.
//
// The framework's `hex repl` CLI uses this against a bare
// environment. Scaffolded applications use it against their own
// container's shared environment, so every Lua module registered by
// framework providers (db, config, cache, log, events, queue, env,
// ai/agent, ...) and by consumer providers (domain services) is
// available at the prompt — the Tinker / Rails console / Phoenix
// IEx pattern for hex apps.
//
// Teal mode uses a persistent tl.init_env so declarations on one
// line remain visible on subsequent lines (see hex/lua/teal.Session).
// Locals still die with their chunk (standard Lua semantics); use
// `global x: T = v` to declare persistent Teal variables.
package repl

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	lua "github.com/yuin/gopher-lua"

	hexlua "github.com/jordanbrauer/hex/lua"
	"github.com/jordanbrauer/hex/lua/teal"
	tuirepl "github.com/jordanbrauer/hex/tui/components/repl"
)

// Mode selects the language the REPL evaluates.
type Mode int

const (
	// ModeTeal evaluates input as Teal by default. This is the
	// framework default — Teal's type-checker catches typos and
	// mistakes in interactive sessions, and Teal source falls
	// through to Lua for expressions that don't need typing.
	ModeTeal Mode = iota

	// ModeLua evaluates input as plain Lua. Looser semantics; no
	// type-checker; implicit globals allowed. Prefer for quick
	// prototyping when Teal's strictness gets in the way.
	ModeLua
)

// String returns the human-readable mode name used in the banner and
// prompt.
func (m Mode) String() string {
	switch m {
	case ModeLua:
		return "lua"
	default:
		return "teal"
	}
}

// Options configures a REPL run.
type Options struct {
	// Mode selects Teal (default) or Lua.
	Mode Mode

	// In, Out, ErrOut are the REPL's I/O streams. Any zero value
	// falls back to the process's os.Stdin/os.Stdout/os.Stderr via
	// the caller; the package does not open OS fds on its own.
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	// AppName appears in the prompt: "<AppName>(teal)> ". Defaults
	// to "hex". Set to your app's binary name (e.g. "myapp") for the
	// scaffolded app REPL.
	AppName string

	// Banner is an optional extra line printed after the standard
	// banner. Callers can use it to warn about the current
	// environment ("connected to PRODUCTION") or advertise available
	// modules.
	Banner string

	// Env, if non-nil, is the hex/lua.Environment the REPL should
	// evaluate against. When nil, Run creates a bare environment.
	// The framework CLI (`hex repl`) passes nil; app-scoped REPLs
	// pass the container's shared environment so registered modules
	// are available.
	Env *hexlua.Environment

	// Interactive selects between the two REPL loops:
	//
	//   false (default) — bufio.Scanner over In/Out/ErrOut. No
	//     terminal features; suitable for pipes, tests, and any
	//     non-TTY caller.
	//
	//   true — Bubble Tea program via hex/tui/components/repl,
	//     giving arrow-key editing, command history, styled
	//     output, and a scrollable viewport. Requires a real TTY;
	//     the caller is responsible for detecting one (typically
	//     via golang.org/x/term or mattn/go-isatty on os.Stdin).
	Interactive bool
}

// Run executes the read-eval-print loop with the given options and
// blocks until the user exits (Ctrl+D, "exit", "quit", ".exit", or
// ".quit") or an unrecoverable I/O error occurs.
//
// Run does not close a caller-provided Env — that's the caller's
// responsibility, since the same env typically outlives many REPL
// sessions (or in the framework case, the whole app process).
func Run(opts Options) error {
	env := opts.Env

	// Bare-env path: create and close a private environment.
	if env == nil {
		env = hexlua.New()
		defer env.Close()
	}

	if opts.AppName == "" {
		opts.AppName = "hex"
	}

	// For Teal mode: initialise a persistent Teal env so declarations
	// on one line remain visible on subsequent lines. Non-REPL
	// contexts (hex run script.tl) intentionally get isolated chunk
	// semantics; the REPL is the exception.
	var tealSession *teal.Session

	isTeal := opts.Mode == ModeTeal
	if isTeal {
		if err := teal.Load(env.L); err != nil {
			return fmt.Errorf("repl: load teal: %w", err)
		}

		session, err := teal.NewSession(env.L)
		if err != nil {
			return fmt.Errorf("repl: teal session: %w", err)
		}

		defer session.Close()

		// Propagate any type stubs registered on the environment
		// (via env.SetType, called by framework providers when they
		// install their Lua modules) so require("db"), require("cache"),
		// etc. typecheck in Teal source.
		types := env.Types()
		for name, source := range types {
			if err := session.AddTypeStub(name, source); err != nil {
				return fmt.Errorf("repl: register type %q: %w", name, err)
			}
		}

		// Pre-declare each registered module as a global on the
		// session's persistent env. This works around Teal's
		// chunk-local scope for locals: `local db = require("db")`
		// on one line vanishes before the next line. Global
		// declarations persist across chunks, so `db.query(...)` on
		// any subsequent line typechecks against the .d.tl stub and
		// runs against the PreloadModule'd runtime module.
		//
		// Deterministic order keeps error messages stable.
		if err := preloadTypedGlobals(env, session, types); err != nil {
			return fmt.Errorf("repl: preload globals: %w", err)
		}

		tealSession = session
	}

	bannerHeader := fmt.Sprintf("%s repl — %s mode. Ctrl+D or \"exit\" to quit.", opts.AppName, opts.Mode)

	bannerLines := []string{bannerHeader}

	if opts.Banner != "" {
		bannerLines = append(bannerLines, opts.Banner)
	}

	if isTeal {
		bannerLines = append(bannerLines,
			"note: framework modules (db, cache, config, log, env, events, queue) are typed globals.",
			"      Your own vars need `global x: T = v` to persist across lines (locals die with the chunk).",
			"      Pass `--mode lua` for looser Lua semantics.",
		)
	}

	prompt := opts.AppName + "(" + opts.Mode.String() + ")> "

	if opts.Interactive {
		return runInteractive(env, tealSession, isTeal, prompt, strings.Join(bannerLines, "\n"))
	}

	return runScripted(env, tealSession, isTeal, opts, prompt, bannerLines)
}

// runScripted is the plain-text loop used for pipes, tests, and any
// non-TTY caller. Reads from opts.In, writes to opts.Out/ErrOut,
// no terminal features.
func runScripted(env *hexlua.Environment, session *teal.Session, isTeal bool, opts Options, prompt string, bannerLines []string) error {
	for _, line := range bannerLines {
		fmt.Fprintln(opts.Out, line)
	}

	scanner := bufio.NewScanner(opts.In)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for {
		fmt.Fprint(opts.Out, prompt)

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}

			fmt.Fprintln(opts.Out)

			return nil
		}

		line := scanner.Text()

		if isExitDirective(line) {
			return nil
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		if err := evalLine(env, opts.Out, line, isTeal, session); err != nil {
			msg := trimTraceback(err.Error())
			fmt.Fprintln(opts.ErrOut, "error:", msg)

			if isTeal {
				hint := tealErrorHint(msg)
				if hint != "" {
					fmt.Fprintln(opts.ErrOut, hint)
				}
			}
		}
	}
}

// runInteractive drives the REPL through hex/tui/components/repl,
// a Bubble Tea component with arrow-key editing, history, and
// styled output. Requires a TTY on stdin.
func runInteractive(env *hexlua.Environment, session *teal.Session, isTeal bool, prompt, banner string) error {
	// evaluator is a synchronous closure the TUI calls for each
	// submitted line. It captures env + session; return values
	// feed the viewport.
	evaluator := func(line string) tuirepl.Result {
		if isExitDirective(line) {
			return tuirepl.Result{Exit: true}
		}

		var outBuf strings.Builder

		if err := evalLine(env, &outBuf, line, isTeal, session); err != nil {
			msg := trimTraceback(err.Error())
			errText := "error: " + msg

			if isTeal {
				if hint := tealErrorHint(msg); hint != "" {
					errText += "\n" + hint
				}
			}

			return tuirepl.Result{Output: strings.TrimRight(outBuf.String(), "\n"), Err: errText}
		}

		return tuirepl.Result{Output: strings.TrimRight(outBuf.String(), "\n")}
	}

	model := tuirepl.New(tuirepl.Options{
		Prompt:       prompt,
		Banner:       banner,
		Evaluator:    evaluator,
		HistoryLimit: 1000,
	})

	prog := tea.NewProgram(model, tea.WithAltScreen())

	_, err := prog.Run()

	return err
}

// tealErrorHint maps a Teal error message to a friendly one-line hint,
// or the empty string when nothing useful applies. Keeps the REPL's
// error output focused: one hint at most, targeted at the specific
// stumble.
func tealErrorHint(msg string) string {
	switch {
	case strings.Contains(msg, "unknown variable"):
		return "hint: prefix with `global` to declare, e.g. `global foo: number = 12`"
	case strings.Contains(msg, "no type information for required module"):
		return "hint: Go-registered modules need a .d.tl type stub for Teal typechecking. Pass `--mode lua` to use them now."
	default:
		return ""
	}
}

// preloadTypedGlobals emits `global <name> = require("<name>")` for
// each registered module, evaluated inside the Teal session so both
// the compile-time type table and the runtime _G[name] are
// populated. Callers can then use each module by bare name (db,
// cache, log, ...) without a require() dance.
func preloadTypedGlobals(env *hexlua.Environment, session *teal.Session, types map[string]string) error {
	// Stable ordering by name so any error surfaces the same module
	// first every run.
	names := make([]string, 0, len(types))
	for name := range types {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		src := fmt.Sprintf("global %s = require(%q)", name, name)

		luaSrc, err := session.Compile(src, "<hex:preload>")
		if err != nil {
			return fmt.Errorf("compile %q: %w", name, err)
		}

		script, err := hexlua.Compile(luaSrc, "<hex:preload>")
		if err != nil {
			return fmt.Errorf("assemble %q: %w", name, err)
		}

		if err := env.Exec(script); err != nil {
			return fmt.Errorf("exec %q: %w", name, err)
		}
	}

	return nil
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
// compile. Prints top-of-stack when the expression form succeeds
// and yields a non-nil value.
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
	// The expression path expects one or more return values on top
	// of the stack after the call.
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
