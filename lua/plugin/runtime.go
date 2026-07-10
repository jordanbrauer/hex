package plugin

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goforj/godump"
	"github.com/spf13/cobra"
	glua "github.com/yuin/gopher-lua"

	loglua "github.com/jordanbrauer/hex/log/lua"
	hexlua "github.com/jordanbrauer/hex/lua"
)

// RuntimeOption configures the per-invocation Environment before a
// plugin entrypoint executes. Consumer apps use this to add bindings
// hex already ships for other Lua consumers, e.g.:
//
//	plugin.WithModule("config", (&configlua.Bindings{Store: store}).Loader)
type RuntimeOption func(*hexlua.Environment)

// WithModule preloads an additional require()-able module for every
// plugin invocation made by the resulting Executor.
func WithModule(name string, loader glua.LGFunction) RuntimeOption {
	return func(e *hexlua.Environment) { e.PreloadModule(name, loader) }
}

// NewRuntimeExecutor returns an Executor that runs each plugin
// entrypoint against a fresh hex/lua Environment — one per
// invocation, so no state leaks between commands. The environment
// comes preloaded with the plugin runtime's module ("disk", "log")
// and globals (argv/argc, cmd, dump, explode, sleep); opts can
// preload additional modules.
func NewRuntimeExecutor(opts ...RuntimeOption) Executor {
	return func(path string, cmd *cobra.Command, args []string) error {
		env := hexlua.New()
		defer func() { _ = env.Close() }()

		env.PreloadModule("disk", diskLoader)
		env.PreloadModule("log", (&loglua.Bindings{}).Loader)

		setArgv(env, args)
		setCmd(env, cmd)
		setDump(env)
		setExplode(env)
		setSleep(env)

		for _, opt := range opts {
			opt(env)
		}

		return env.ExecFile(path)
	}
}

// -- globals -----------------------------------------------------------------

func setArgv(env *hexlua.Environment, args []string) {
	l := env.L
	t := l.NewTable()

	for _, a := range args {
		t.Append(glua.LString(a))
	}

	l.SetGlobal("argv", t)
	l.SetGlobal("argc", glua.LNumber(len(args)))
}

func setCmd(env *hexlua.Environment, cmd *cobra.Command) {
	if cmd == nil {
		return
	}

	l := env.L
	t := l.NewTable()

	l.SetField(t, "name", glua.LString(cmd.Name()))
	l.SetField(t, "path", glua.LString(cmd.CommandPath()))
	l.SetFuncs(t, map[string]glua.LGFunction{
		"flags": flagsFn(cmd),
	})
	l.SetGlobal("cmd", t)
}

// flagsFn returns the cmd.flags() constructor: a table of flag
// accessors closing over cmd's *pflag.FlagSet.
func flagsFn(cmd *cobra.Command) glua.LGFunction {
	return func(l *glua.LState) int {
		t := l.NewTable()

		l.SetFuncs(t, map[string]glua.LGFunction{
			"changed": func(l *glua.LState) int {
				l.Push(glua.LBool(cmd.Flags().Changed(l.CheckString(1))))

				return 1
			},
			"has_flags": func(l *glua.LState) int {
				l.Push(glua.LBool(cmd.Flags().HasFlags()))

				return 1
			},
			"get_boolean": func(l *glua.LState) int {
				v, err := cmd.Flags().GetBool(l.CheckString(1))
				if err != nil {
					l.Push(glua.LNil)

					return 1
				}

				l.Push(glua.LBool(v))

				return 1
			},
			"get_boolean_arr": func(l *glua.LState) int {
				v, err := cmd.Flags().GetBoolSlice(l.CheckString(1))
				if err != nil {
					l.Push(glua.LNil)

					return 1
				}

				out := l.NewTable()
				for _, b := range v {
					out.Append(glua.LBool(b))
				}

				l.Push(out)

				return 1
			},
			"get_string": func(l *glua.LState) int {
				v, err := cmd.Flags().GetString(l.CheckString(1))
				if err != nil {
					l.Push(glua.LNil)

					return 1
				}

				l.Push(glua.LString(v))

				return 1
			},
			"get_string_arr": func(l *glua.LState) int {
				v, err := cmd.Flags().GetStringSlice(l.CheckString(1))
				if err != nil {
					l.Push(glua.LNil)

					return 1
				}

				out := l.NewTable()
				for _, s := range v {
					out.Append(glua.LString(s))
				}

				l.Push(out)

				return 1
			},
			"get_number": func(l *glua.LState) int {
				v, err := cmd.Flags().GetFloat64(l.CheckString(1))
				if err != nil {
					l.Push(glua.LNil)

					return 1
				}

				l.Push(glua.LNumber(v))

				return 1
			},
			"get_number_arr": func(l *glua.LState) int {
				v, err := cmd.Flags().GetFloat64Slice(l.CheckString(1))
				if err != nil {
					l.Push(glua.LNil)

					return 1
				}

				out := l.NewTable()
				for _, n := range v {
					out.Append(glua.LNumber(n))
				}

				l.Push(out)

				return 1
			},
		})

		l.Push(t)

		return 1
	}
}

var dumper = godump.NewDumper(godump.WithoutHeader())

// setDump installs the `dump(value)` global: pretty-prints a Lua
// value (tables recursively) to stdout for debugging, headed by the
// calling script's source location.
func setDump(env *hexlua.Environment) {
	env.L.SetGlobal("dump", env.L.NewFunction(func(l *glua.LState) int {
		v := l.CheckAny(1)

		if dbg, ok := l.GetStack(1); ok {
			_, _ = l.GetInfo("Sl", dbg, glua.LNil)
			fmt.Printf("<#dump // %s:%d\n", dbg.Source, dbg.CurrentLine)
		}

		dumper.Dump(luaToGoValue(v))

		return 0
	}))
}

// luaToGoValue recursively converts a Lua value into a native Go
// value for dump's pretty-printer. Sequential-integer-keyed tables
// become []any; anything else becomes map[string]any.
func luaToGoValue(v glua.LValue) any {
	t, ok := v.(*glua.LTable)
	if !ok {
		switch cv := v.(type) {
		case glua.LBool:
			return bool(cv)
		case glua.LNumber:
			return float64(cv)
		case glua.LString:
			return string(cv)
		case *glua.LNilType:
			return nil
		default:
			return v.String()
		}
	}

	maxn := t.MaxN()
	if maxn > 0 && maxn == t.Len() {
		arr := make([]any, 0, maxn)
		for i := 1; i <= maxn; i++ {
			arr = append(arr, luaToGoValue(t.RawGetInt(i)))
		}

		return arr
	}

	m := map[string]any{}
	t.ForEach(func(k, val glua.LValue) {
		m[k.String()] = luaToGoValue(val)
	})

	return m
}

// setExplode installs `explode(s, sep)`, splitting s on sep into a
// Lua array table.
func setExplode(env *hexlua.Environment) {
	env.L.SetGlobal("explode", env.L.NewFunction(func(l *glua.LState) int {
		t := l.NewTable()

		for _, seg := range strings.Split(l.ToString(1), l.ToString(2)) {
			t.Append(glua.LString(seg))
		}

		l.Push(t)

		return 1
	}))
}

// setSleep installs `sleep(ms)`, blocking for the given number of
// milliseconds.
func setSleep(env *hexlua.Environment) {
	env.L.SetGlobal("sleep", env.L.NewFunction(func(l *glua.LState) int {
		time.Sleep(time.Millisecond * time.Duration(l.ToInt(1)))

		return 0
	}))
}

// -- disk module ---------------------------------------------------------

// diskFunctions backs require("disk"): the file-writing primitive
// plugin scripts use to generate files.
var diskFunctions = map[string]glua.LGFunction{
	"append": func(l *glua.LState) int {
		n, err := diskAppend(l.CheckString(1), []byte(l.CheckString(2)))

		return pushWriteResult(l, n, err)
	},
	"write": func(l *glua.LState) int {
		n, err := diskWrite(l.CheckString(1), []byte(l.CheckString(2)))

		return pushWriteResult(l, n, err)
	},
	"erase": func(l *glua.LState) int {
		return pushErrResult(l, os.RemoveAll(l.CheckString(1)))
	},
	"exists": func(l *glua.LState) int {
		_, err := os.Stat(l.CheckString(1))
		l.Push(glua.LBool(err == nil))

		return 1
	},
	"mkdir": func(l *glua.LState) int {
		return pushErrResult(l, os.MkdirAll(l.CheckString(1), 0o755))
	},
	"touch": func(l *glua.LState) int {
		return pushErrResult(l, diskTouch(l.CheckString(1)))
	},
}

func diskLoader(l *glua.LState) int {
	t := l.NewTable()
	l.SetFuncs(t, diskFunctions)
	l.Push(t)

	return 1
}

func pushWriteResult(l *glua.LState, n int, err error) int {
	l.Push(glua.LNumber(n))

	if err != nil {
		l.Push(glua.LString(err.Error()))
	} else {
		l.Push(glua.LNil)
	}

	return 2
}

func pushErrResult(l *glua.LState, err error) int {
	if err != nil {
		l.Push(glua.LString(err.Error()))
	} else {
		l.Push(glua.LNil)
	}

	return 1
}

func diskWrite(path string, content []byte) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	return f.Write(content)
}

func diskAppend(path string, content []byte) (int, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	return f.Write(content)
}

func diskTouch(path string) error {
	if _, err := os.Stat(path); err == nil {
		now := time.Now()

		return os.Chtimes(path, now, now)
	}

	_, err := diskWrite(path, nil)

	return err
}
