package repl

import (
	"strings"
	"testing"

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
