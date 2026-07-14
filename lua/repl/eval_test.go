package repl

import (
	"strings"
	"testing"

	glua "github.com/yuin/gopher-lua"

	hexlua "github.com/jordanbrauer/hex/lua"
)

func TestEvalFennel(t *testing.T) {
	env := hexlua.New()
	defer env.Close()

	out, incomplete, err := Eval(env, ModeFennel, `(+ 1 2)`)
	if err != nil || incomplete {
		t.Fatalf("err=%v incomplete=%v", err, incomplete)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("out = %q", out)
	}

	// print() capture
	out, _, err = Eval(env, ModeFennel, `(print "hello")`)
	if err != nil || !strings.Contains(out, "hello") {
		t.Errorf("out=%q err=%v", out, err)
	}

	// incomplete input buffers
	_, incomplete, err = Eval(env, ModeFennel, `(fn foo []`)
	if err != nil || !incomplete {
		t.Errorf("incomplete=%v err=%v", incomplete, err)
	}

	// errors come back trimmed
	_, _, err = Eval(env, ModeFennel, `(error "boom")`)
	if err == nil || strings.Contains(err.Error(), "stack traceback") {
		t.Errorf("err = %v", err)
	}
}

func TestEvalLua(t *testing.T) {
	env := hexlua.New()
	defer env.Close()

	out, _, err := Eval(env, ModeLua, `1 + 41`)
	if err != nil || !strings.Contains(out, "42") {
		t.Errorf("out=%q err=%v", out, err)
	}
}

func TestConsoleFennelGlobalsPersist(t *testing.T) {
	env := hexlua.New()
	defer env.Close()

	console, err := NewConsole(env, ModeFennel)
	if err != nil {
		t.Fatal(err)
	}
	defer console.Close()

	if _, _, err := console.Eval(`(global answer 41)`); err != nil {
		t.Fatal(err)
	}

	out, _, err := console.Eval(`(+ answer 1)`)
	if err != nil || !strings.Contains(out, "42") {
		t.Errorf("out=%q err=%v", out, err)
	}
}

func TestConsoleTealDeclarationsPersist(t *testing.T) {
	env := hexlua.New()
	defer env.Close()

	console, err := NewConsole(env, ModeTeal)
	if err != nil {
		t.Fatal(err)
	}
	defer console.Close()

	if _, _, err := console.Eval(`global count: integer = 41`); err != nil {
		t.Fatalf("declare: %v", err)
	}

	out, _, err := console.Eval(`count + 1`)
	if err != nil || !strings.Contains(out, "42") {
		t.Errorf("out=%q err=%v", out, err)
	}
}

func TestConsoleTealSeesTypeStubs(t *testing.T) {
	env := hexlua.New()
	defer env.Close()

	env.SetType("greeter", `
		local record greeter
			hello: function(): string
		end
		return greeter
	`)
	env.PreloadModule("greeter", func(L *glua.LState) int {
		mod := L.NewTable()
		L.SetField(mod, "hello", L.NewFunction(func(L *glua.LState) int {
			L.Push(glua.LString("hi"))
			return 1
		}))
		L.Push(mod)
		return 1
	})

	console, err := NewConsole(env, ModeTeal)
	if err != nil {
		t.Fatal(err)
	}
	defer console.Close()

	// The typed module was pre-declared as a global by NewConsole —
	// no require() needed, same as Run.
	out, _, err := console.Eval(`greeter.hello()`)
	if err != nil || !strings.Contains(out, "hi") {
		t.Errorf("out=%q err=%v", out, err)
	}
}
