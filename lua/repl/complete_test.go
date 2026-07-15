package repl

import (
	"reflect"
	"testing"

	glua "github.com/yuin/gopher-lua"

	hexlua "github.com/jordanbrauer/hex/lua"
)

func TestParseCompletionContext(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		cursor     int
		wantChain  []string
		wantPrefix string
		wantPStart int
	}{
		{"bare identifier", "pri", 3, nil, "pri", 0},
		{"empty at start", "", 0, nil, "", 0},
		{"member access", "db.q", 4, []string{"db"}, "q", 3},
		{"chained member", "a.b.c", 5, []string{"a", "b"}, "c", 4},
		{"just a dot", "db.", 3, []string{"db"}, "", 3},
		{"after non-ident", "foo(", 4, nil, "", 4},
		{"call arg identifier", "foo(bar", 7, nil, "bar", 4},
		{"call arg member", "foo(bar.b", 9, []string{"bar"}, "b", 8},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chain, prefix, ps := parseCompletionContext(tc.input, tc.cursor)

			if !reflect.DeepEqual(chain, tc.wantChain) {
				t.Errorf("chain = %v, want %v", chain, tc.wantChain)
			}

			if prefix != tc.wantPrefix {
				t.Errorf("prefix = %q, want %q", prefix, tc.wantPrefix)
			}

			if ps != tc.wantPStart {
				t.Errorf("prefixStart = %d, want %d", ps, tc.wantPStart)
			}
		})
	}
}

func TestGlobalCandidates(t *testing.T) {
	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	// Add some app-visible globals.
	env.L.SetGlobal("query_users", glua.LString("marker"))
	env.L.SetGlobal("query_orders", glua.LString("marker"))
	env.L.SetGlobal("greet", glua.LString("marker"))

	cands := globalCandidates(env.L, "query")

	names := make([]string, 0, len(cands))
	for _, c := range cands {
		names = append(names, c.Text)
	}

	// Should include our two matches, alphabetized. Stdlib may add
	// other "query"-prefixed names, but our seeds should be present.
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}

	if !found["query_orders"] || !found["query_users"] {
		t.Errorf("missing seed globals; got %v", names)
	}

	if found["greet"] {
		t.Errorf("prefix mismatch admitted 'greet' into query* candidates: %v", names)
	}
}

func TestGlobalCandidates_hidesUnderscoreNames(t *testing.T) {
	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	env.L.SetGlobal("_private", glua.LString("marker"))
	env.L.SetGlobal("public", glua.LString("marker"))

	cands := globalCandidates(env.L, "")
	for _, c := range cands {
		if c.Text == "_private" {
			t.Errorf("underscore-prefixed name should be hidden: %s", c.Text)
		}
	}
}

func TestMemberCandidates(t *testing.T) {
	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	// Build a fake `db` module with a few methods.
	tbl := env.L.NewTable()
	env.L.SetField(tbl, "query", env.L.NewFunction(func(*glua.LState) int { return 0 }))
	env.L.SetField(tbl, "queryOne", env.L.NewFunction(func(*glua.LState) int { return 0 }))
	env.L.SetField(tbl, "exec", env.L.NewFunction(func(*glua.LState) int { return 0 }))
	env.L.SetGlobal("db", tbl)

	cands := memberCandidates(env.L, []string{"db"}, "q")

	names := make([]string, 0, len(cands))
	for _, c := range cands {
		names = append(names, c.Text)
	}

	want := []string{"query", "queryOne"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("cands = %v, want %v", names, want)
	}
}

func TestMemberCandidates_missingReceiver(t *testing.T) {
	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	cands := memberCandidates(env.L, []string{"nonexistent"}, "")
	if len(cands) != 0 {
		t.Errorf("expected nil, got %v", cands)
	}
}

func TestMemberCandidates_receiverNotTable(t *testing.T) {
	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	env.L.SetGlobal("scalar", glua.LNumber(42))

	cands := memberCandidates(env.L, []string{"scalar"}, "")
	if len(cands) != 0 {
		t.Errorf("scalar receiver should yield no candidates: %v", cands)
	}
}

func TestCompleter_endToEnd(t *testing.T) {
	env := hexlua.New()
	t.Cleanup(func() { _ = env.Close() })

	tbl := env.L.NewTable()
	env.L.SetField(tbl, "query", env.L.NewFunction(func(*glua.LState) int { return 0 }))
	env.L.SetField(tbl, "queryOne", env.L.NewFunction(func(*glua.LState) int { return 0 }))
	env.L.SetGlobal("db", tbl)

	comp := Completer(env)

	// "db.q" cursor at 4 → should return {query, queryOne} with prefixStart=3.
	cands, prefixStart := comp("teal", "db.q", 4)

	if prefixStart != 3 {
		t.Errorf("prefixStart = %d, want 3", prefixStart)
	}

	if len(cands) != 2 {
		t.Fatalf("cands = %d, want 2 (%v)", len(cands), cands)
	}

	if cands[0].Text != "query" || cands[1].Text != "queryOne" {
		t.Errorf("cands = %v, want [query queryOne]", cands)
	}
}
