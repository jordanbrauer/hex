package repl

import (
	"sort"
	"strings"

	glua "github.com/yuin/gopher-lua"

	hexlua "github.com/jordanbrauer/hex/lua"
	tuirepl "github.com/jordanbrauer/hex/tui/components/repl"
)

// completer returns a tuirepl.Completer that inspects the Lua state's
// globals table to enumerate candidates. Called on every Tab key
// press in the interactive REPL.
//
// Recognised contexts (walked backwards from the cursor):
//
//	pri<TAB>          bare identifier prefix "pri" \u2192 all globals
//	                   starting with "pri" (print, primary_db, ...)
//
//	db.q<TAB>         member access: get _G.db, list keys starting
//	                   with "q" (query, queryOne).
//
//	obj.a.b<TAB>      chained access: walk _G.obj.a, list keys.
//
// Only string keys on the receiver table are candidates. Numeric
// keys, metatables, and special __index values are ignored (v1).
func completer(env *hexlua.Environment) tuirepl.Completer {
	return func(mode, input string, cursorPos int) ([]tuirepl.Candidate, int) {
		if cursorPos > len(input) {
			cursorPos = len(input)
		}

		chain, prefix, prefixStart := parseCompletionContext(input, cursorPos)

		var cands []tuirepl.Candidate

		if len(chain) == 0 {
			cands = globalCandidates(env.L, prefix)
		} else {
			cands = memberCandidates(env.L, chain, prefix)
		}

		return cands, prefixStart
	}
}

// parseCompletionContext walks backward from cursorPos to determine
// what the user is currently typing. Returns:
//
//	chain       the dotted receiver chain (empty for bare identifier)
//	prefix      the token fragment right of the cursor's dot (or
//	            the whole identifier if no dot)
//	prefixStart the byte offset where prefix begins in input
//
// Examples (cursor at end):
//
//	"pri"      \u2192 chain=[], prefix="pri", prefixStart=0
//	"db.q"     \u2192 chain=[db], prefix="q", prefixStart=3
//	"a.b.c"    \u2192 chain=[a,b], prefix="c", prefixStart=4
//	"foo("     \u2192 chain=[], prefix="", prefixStart=4
func parseCompletionContext(input string, cursorPos int) (chain []string, prefix string, prefixStart int) {
	// Walk left from cursor while chars are part of an identifier
	// chain (letters, digits, underscore, dot).
	start := cursorPos
	for start > 0 {
		c := input[start-1]
		if !isIdentChar(c) && c != '.' {
			break
		}

		start--
	}

	token := input[start:cursorPos]

	if !strings.Contains(token, ".") {
		return nil, token, start
	}

	parts := strings.Split(token, ".")

	// All parts before the last are the receiver chain; the last
	// part (possibly empty) is the prefix being completed.
	chain = parts[:len(parts)-1]
	prefix = parts[len(parts)-1]

	// prefixStart is right after the final dot.
	prefixStart = start + len(token) - len(prefix)

	return chain, prefix, prefixStart
}

// isIdentChar reports whether c can appear inside a Lua identifier.
func isIdentChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '_':
		return true
	}

	return false
}

// globalCandidates enumerates keys in the LState's globals table
// (_G) whose names start with prefix. Sorted alphabetically for
// deterministic cycling order.
func globalCandidates(L *glua.LState, prefix string) []tuirepl.Candidate {
	globals := L.Get(glua.GlobalsIndex).(*glua.LTable)

	return tableCandidates(globals, prefix)
}

// memberCandidates walks the receiver chain from _G and enumerates
// string keys on the final table. Returns nil if any step of the
// chain isn't a table (or doesn't exist).
func memberCandidates(L *glua.LState, chain []string, prefix string) []tuirepl.Candidate {
	current := L.Get(glua.GlobalsIndex)

	for _, name := range chain {
		tbl, ok := current.(*glua.LTable)
		if !ok {
			return nil
		}

		current = tbl.RawGetString(name)
		if current == glua.LNil {
			return nil
		}
	}

	tbl, ok := current.(*glua.LTable)
	if !ok {
		return nil
	}

	return tableCandidates(tbl, prefix)
}

// tableCandidates collects string keys from tbl that start with
// prefix and returns them as Candidates sorted alphabetically.
// Skips keys starting with '_' (Lua convention for private / meta).
func tableCandidates(tbl *glua.LTable, prefix string) []tuirepl.Candidate {
	var names []string

	tbl.ForEach(func(k, v glua.LValue) {
		key, ok := k.(glua.LString)
		if !ok {
			return
		}

		name := string(key)

		// Hide names starting with underscore \u2014 typically internals
		// (_G, _ENV, _hex_*, __*).
		if strings.HasPrefix(name, "_") {
			return
		}

		if prefix != "" && !strings.HasPrefix(name, prefix) {
			return
		}

		names = append(names, name)
	})

	sort.Strings(names)

	cands := make([]tuirepl.Candidate, 0, len(names))
	for _, n := range names {
		cands = append(cands, tuirepl.Candidate{Text: n})
	}

	return cands
}
