// Package lua embeds a Lua runtime for hex applications using
// github.com/yuin/gopher-lua.
//
// hex/lua is intentionally minimal (ADR-0007): it exposes an Environment
// that compiles and executes Lua scripts and gives consumers access to
// the underlying *lua.LState so they can install whatever Go→Lua
// bindings they need. hex ships no modules, no plugin system, no
// discovery convention. Those live in consumer apps.
//
// Compilation is separated from execution so scripts can be compiled once
// and run many times against many environments — the typical pattern
// for load testing (see zk/lemming) or plugin dispatch.
//
// Example:
//
//	env := lua.New()
//	defer env.Close()
//
//	// Install a consumer-owned Go→Lua binding.
//	env.PreloadModule("http", myhttp.Loader)
//
//	// Set a global.
//	if err := env.SetGlobal("build_version", "v1.2.3"); err != nil {
//	    return err
//	}
//
//	// Compile once, execute many times.
//	script, err := env.Compile("print(build_version)", "hello.lua")
//	if err != nil { return err }
//	if err := env.Exec(script); err != nil { return err }
//
// The zero value is not usable; call New.
package lua

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/jordanbrauer/hex/lua/teal"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

// Environment is a single Lua VM with hex-friendly lifecycle helpers.
type Environment struct {
	// L is the underlying gopher-lua state. Exported so consumers can
	// install modules, register types, and perform advanced operations
	// that hex/lua does not wrap. Callers should treat direct L access as
	// an escape hatch, not the primary API.
	L *lua.LState

	// types maps module name to its Teal .d.tl source. Populated via
	// SetType; read by hex/lua/teal.Session on session init so
	// require("name") in Teal source finds the module's type signature.
	types map[string]string

	// stdout is where the overridden Lua `print` writes. Defaults to
	// os.Stdout. The REPL redirects this per-eval so print output
	// flows through Result.Output instead of clobbering the Bubble
	// Tea render.
	stdout io.Writer

	closed   bool
	tealOnce sync.Once
	tealErr  error
}

// Option configures a new Environment.
type Option func(*envConfig)

type envConfig struct {
	openLibs    bool
	callStack   int
	registrySz  int
	skipOpenAll bool
	packagePath []string
}

// WithoutStandardLibraries opens the Environment without gopher-lua's
// default stdlib (io, os, debug, etc). Use for locked-down sandboxes.
func WithoutStandardLibraries() Option {
	return func(c *envConfig) { c.skipOpenAll = true }
}

// WithCallStackSize sets the maximum Lua call-stack depth. Default is
// gopher-lua's default (~256). Increase for deeply recursive scripts.
func WithCallStackSize(n int) Option {
	return func(c *envConfig) { c.callStack = n }
}

// WithRegistrySize sets the initial size of the Lua registry. Default is
// gopher-lua's default. Tune if you know you will register many values.
func WithRegistrySize(n int) Option {
	return func(c *envConfig) { c.registrySz = n }
}

// WithPackagePath appends directories to Lua's `package.path` so scripts
// can `require("x")` files under those directories. Directories are
// searched in the order given, before Lua's default paths.
func WithPackagePath(dirs ...string) Option {
	return func(c *envConfig) { c.packagePath = append(c.packagePath, dirs...) }
}

// New returns a fresh Environment. Options tune the VM; the zero-argument
// call is safe and produces a Lua state with stdlib loaded.
func New(opts ...Option) *Environment {
	cfg := envConfig{openLibs: true}

	for _, opt := range opts {
		opt(&cfg)
	}

	stateOpts := lua.Options{
		SkipOpenLibs: cfg.skipOpenAll,
	}

	if cfg.callStack > 0 {
		stateOpts.CallStackSize = cfg.callStack
	}

	if cfg.registrySz > 0 {
		stateOpts.RegistrySize = cfg.registrySz
	}

	L := lua.NewState(stateOpts)

	if len(cfg.packagePath) > 0 {
		if err := prependPackagePath(L, cfg.packagePath); err != nil {
			// Package path modifications should never fail on a fresh
			// state, so panic here — if it does fail, we have a much
			// bigger problem than an error return value would convey.
			L.Close()

			panic(fmt.Sprintf("lua: configure package.path: %v", err))
		}
	}

	e := &Environment{L: L, stdout: os.Stdout}

	// Only override print if stdlib is loaded — WithoutStandardLibraries
	// callers expect a locked-down state where print isn't available.
	if !cfg.skipOpenAll {
		e.installPrint()
	}

	return e
}

// installPrint replaces gopher-lua's default print (hardcoded to
// fmt.Print / os.Stdout) with one that writes to e.stdout, so
// callers can redirect script output at any time via SetStdout.
func (e *Environment) installPrint() {
	e.L.SetGlobal("print", e.L.NewFunction(func(L *lua.LState) int {
		top := L.GetTop()
		for i := 1; i <= top; i++ {
			fmt.Fprint(e.stdout, L.ToStringMeta(L.Get(i)).String())

			if i != top {
				fmt.Fprint(e.stdout, "\t")
			}
		}

		fmt.Fprintln(e.stdout, "")

		return 0
	}))
}

// SetStdout redirects the Lua `print` function's output. The change is
// live: subsequent print() calls write to w. Nil restores os.Stdout.
func (e *Environment) SetStdout(w io.Writer) {
	if w == nil {
		e.stdout = os.Stdout

		return
	}

	e.stdout = w
}

// Stdout returns the current writer that print() flushes to.
func (e *Environment) Stdout() io.Writer { return e.stdout }

// Close releases the Lua state's resources. Safe to call more than once.
// After Close, the Environment must not be used.
func (e *Environment) Close() error {
	if e == nil || e.closed {
		return nil
	}

	e.L.Close()
	e.closed = true

	return nil
}

// PreloadModule registers a module loader under name. When a Lua script
// calls `require("name")`, loader is invoked to push the module table
// onto the stack. Matches gopher-lua's L.PreloadModule but hides that
// detail so consumers do not need to type-assert.
func (e *Environment) PreloadModule(name string, loader lua.LGFunction) {
	e.L.PreloadModule(name, loader)
}

// SetType registers a Teal .d.tl source describing the shape of a
// module. hex/lua/teal.Session reads these at session init and
// exposes them via package.path so require("name") typechecks in
// Teal source.
//
// This is compile-time metadata only — it has no effect on the
// runtime module (registered via PreloadModule) and is silently
// ignored when running Lua directly (Lua doesn't typecheck).
func (e *Environment) SetType(moduleName, tealSource string) {
	if e.types == nil {
		e.types = map[string]string{}
	}

	e.types[moduleName] = tealSource
}

// Types returns a copy of the registered type stubs. Consumers who
// want the underlying map (to mutate) should use SetType.
func (e *Environment) Types() map[string]string {
	if e.types == nil {
		return nil
	}

	out := make(map[string]string, len(e.types))
	for k, v := range e.types {
		out[k] = v
	}

	return out
}

// SetGlobal installs value as a Lua global. Supported Go types: string,
// bool, int/int64/uint64, float64, nil, and any lua.LValue. Unsupported
// types return an error.
func (e *Environment) SetGlobal(name string, value any) error {
	lv, err := toLuaValue(e.L, value)
	if err != nil {
		return fmt.Errorf("lua: set global %q: %w", name, err)
	}

	e.L.SetGlobal(name, lv)

	return nil
}

// GetGlobal returns the Lua value at name. If the global is unset, the
// result is lua.LNil.
func (e *Environment) GetGlobal(name string) lua.LValue {
	return e.L.GetGlobal(name)
}

// -- Compilation & execution ----------------------------------------------

// Script is a compiled Lua chunk that can be executed against any
// Environment. The same Script is safe to share across environments
// (each Exec re-attaches it) but not safe for concurrent Exec calls on
// the same Environment; use one Environment per goroutine.
type Script struct {
	proto *lua.FunctionProto
	name  string
}

// Name returns the script's source name (usually the filename).
func (s *Script) Name() string {
	if s == nil {
		return ""
	}

	return s.name
}

// Compile parses source into a Script. name is used in error messages and
// Lua stack traces; pass the file path or a synthetic name like
// "inline". Compile does not touch the Environment; you can compile once
// and share the Script across many environments.
func Compile(source, name string) (*Script, error) {
	reader := strings.NewReader(source)

	chunk, err := parse.Parse(reader, name)
	if err != nil {
		return nil, fmt.Errorf("lua: parse %s: %w", name, err)
	}

	proto, err := lua.Compile(chunk, name)
	if err != nil {
		return nil, fmt.Errorf("lua: compile %s: %w", name, err)
	}

	return &Script{proto: proto, name: name}, nil
}

// CompileFile reads path and compiles its contents. The file's absolute
// path is used as the script name.
func CompileFile(path string) (*Script, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("lua: read %s: %w", path, err)
	}

	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		abs = path
	}

	return Compile(string(data), abs)
}

// Exec runs a compiled script against the environment. Any Lua panic is
// converted into an error with the stack trace attached.
func (e *Environment) Exec(script *Script) error {
	if script == nil || script.proto == nil {
		return errors.New("lua: nil script")
	}

	defer func() {
		if r := recover(); r != nil {
			// gopher-lua's PCall already traps most panics; a bare panic
			// here indicates a bug in a Go binding or the runtime itself.
			// Re-panic with a wrapped stack so the caller sees it.
			panic(fmt.Errorf("lua: exec panic: %v\n%s", r, debug.Stack()))
		}
	}()

	fn := e.L.NewFunctionFromProto(script.proto)
	e.L.Push(fn)

	if err := e.L.PCall(0, lua.MultRet, nil); err != nil {
		return fmt.Errorf("lua: exec %s: %w", script.name, err)
	}

	return nil
}

// ExecString compiles source once and executes it. For scripts that will
// run many times, prefer Compile + Exec to skip the parse phase.
func (e *Environment) ExecString(source, name string) error {
	script, err := Compile(source, name)
	if err != nil {
		return err
	}

	return e.Exec(script)
}

// ExecFile compiles and executes a Lua or Teal file. .tl files are
// transpiled through the embedded Teal compiler (see hex/lua/teal)
// before being handed to gopher-lua. Teal support is lazily loaded on
// the first .tl encountered per Environment, so pure-Lua users pay
// nothing.
func (e *Environment) ExecFile(path string) error {
	script, err := e.LoadFile(path)
	if err != nil {
		return err
	}

	return e.Exec(script)
}

// LoadFile reads path and compiles it to a Script, auto-detecting
// Lua vs Teal by extension. Same semantics as CompileFile for .lua;
// .tl files first run through the Teal compiler.
func (e *Environment) LoadFile(path string) (*Script, error) {
	isTL := filepath.Ext(path) == ".tl"

	if !isTL {
		return CompileFile(path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("lua: read %s: %w", path, err)
	}

	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		abs = path
	}

	return e.LoadString(string(data), abs, true)
}

// LoadString compiles a source string to a Script, treating it as
// Teal when isTeal is true (routed through the embedded Teal
// compiler) or Lua otherwise.
func (e *Environment) LoadString(source, name string, isTeal bool) (*Script, error) {
	if !isTeal {
		return Compile(source, name)
	}

	if err := e.ensureTeal(); err != nil {
		return nil, err
	}

	luaSrc, err := teal.Compile(e.L, source, name)
	if err != nil {
		return nil, fmt.Errorf("lua: compile teal %s: %w", name, err)
	}

	return Compile(luaSrc, name)
}

// ensureTeal lazy-loads the Teal compiler into this Environment's
// state. sync.Once so concurrent callers don't race + repeated calls
// are free.
func (e *Environment) ensureTeal() error {
	e.tealOnce.Do(func() {
		e.tealErr = teal.Load(e.L)
	})

	return e.tealErr
}

// CheckFile runs a Lua or Teal file through validation without
// executing it. For .lua files this parses only. For .tl files this
// runs the Teal type-checker.
//
// Intended for CI (fail the build on Teal type errors) and for
// pre-flight validation of user-supplied scripts.
func (e *Environment) CheckFile(path string) error {
	isTL := filepath.Ext(path) == ".tl"

	if !isTL {
		_, err := CompileFile(path)

		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("lua: read %s: %w", path, err)
	}

	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		abs = path
	}

	return e.CheckString(string(data), abs, true)
}

// CheckString validates source without executing it. When isTeal is
// true the source runs through the Teal typechecker; otherwise it
// runs through Lua's parser.
func (e *Environment) CheckString(source, name string, isTeal bool) error {
	if !isTeal {
		_, err := Compile(source, name)

		return err
	}

	if err := e.ensureTeal(); err != nil {
		return err
	}

	return teal.Check(e.L, source, name)
}

// -- helpers ---------------------------------------------------------------

// toLuaValue converts common Go primitives into gopher-lua values.
func toLuaValue(L *lua.LState, v any) (lua.LValue, error) {
	switch x := v.(type) {
	case nil:
		return lua.LNil, nil
	case string:
		return lua.LString(x), nil
	case bool:
		return lua.LBool(x), nil
	case int:
		return lua.LNumber(x), nil
	case int64:
		return lua.LNumber(x), nil
	case uint64:
		return lua.LNumber(x), nil
	case float64:
		return lua.LNumber(x), nil
	case lua.LValue:
		return x, nil
	case *lua.LTable:
		return x, nil
	default:
		return nil, fmt.Errorf("unsupported Go type %T (pass lua.LValue for complex types)", v)
	}
}

// prependPackagePath adds dirs to the front of package.path so that
// require() looks in them first.
func prependPackagePath(L *lua.LState, dirs []string) error {
	pkg := L.GetGlobal("package")

	tbl, ok := pkg.(*lua.LTable)
	if !ok {
		return errors.New("package global is not a table")
	}

	currentPath := tbl.RawGetString("path").String()

	added := make([]string, 0, 2*len(dirs))
	for _, d := range dirs {
		added = append(added,
			filepath.Join(d, "?.lua"),
			filepath.Join(d, "?", "init.lua"),
		)
	}

	newPath := strings.Join(added, ";")
	if currentPath != "" {
		newPath = newPath + ";" + currentPath
	}

	tbl.RawSetString("path", lua.LString(newPath))

	return nil
}
