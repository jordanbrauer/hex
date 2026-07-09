# Design: Tree-sitter integration

**Status:** Draft — scoping. Not yet an ADR.
**Author:** Jordan
**Date:** 2026-07-09

## Context

hex ships first-class Lua and Teal support (embedded compiler, typed
REPL, `.d.tl` stubs). The REPL is functional but plain — no syntax
highlighting, no structural intelligence. Users see monochrome input
and monochrome echoes.

Beyond aesthetics, several planned features want structural knowledge
of the source:

- **Tab completion** — needs to know the cursor is inside an identifier,
  behind a `.`, or inside a string
- **Jump-to-definition** in `.tl` script files
- **Hex/log source-line rendering** could use highlighted snippets
- **`hex check`** could show highlighted error contexts
- **CLI code formatters / templates** could reuse the same tooling

The common substrate all these want is a **fast, incremental,
error-tolerant parser** with **language-agnostic query facilities**.
That's tree-sitter.

## Goals

- **Syntax highlighting** in the REPL — both as-you-type in the input
  line and highlight-on-echo in scrollback.
- **Foundation for tab completion** — expose enough of the parse tree
  that a completer can ask "what token is at cursor position N?"
  "what's the enclosing scope?" "is this a method call receiver?".
- **Reusable across contexts** — REPL, `hex run`, `hex check`, any
  future editor-integration surface.
- **Opt-in per language** — consumers who only use Lua don't pay
  cost for Teal grammar and vice versa.
- **Consistent theme** — a single hex-owned highlight palette across
  the framework's Lua/Teal surfaces.

## Non-goals (v1)

- Language server implementation (no LSP wire protocol).
- Cross-file symbol resolution beyond a single script buffer.
- Refactoring / code transformation.
- Grammars for languages other than Lua and Teal (initially).
- Bundled `.wasm` grammars for editor integrations (that's the
  editor's job — we ship the Go side).

## Proposed architecture

### Package layout

```
hex/tree-sitter/                thin wrapper over go-tree-sitter
  ├── parser.go                 Parser, Tree, Node abstractions
  ├── highlight.go              Highlight query loading + span extraction
  ├── theme.go                  Default palette + Style resolver
  └── lua/                      Lua grammar package
      └── highlights.scm        embedded query file
  └── teal/                     Teal grammar package
      └── highlights.scm        embedded query file
```

Each grammar package is opt-in. Consumers import
`hex/tree-sitter/lua` only if they need Lua highlighting; the cgo
weight and grammar binary only ship in binaries that use them.

### Dependency choice

**Preferred: `github.com/smacker/go-tree-sitter`**

- Mature Go wrapper (2020+, well-maintained)
- Bundled grammars for common languages including Lua
- Query engine matches tree-sitter's canonical query syntax
- Requires cgo

**Considered but not chosen: pure-Go alternatives**

There is no complete pure-Go implementation of tree-sitter's
incremental algorithm. Some ports exist (mnemsyne, others) but
lack query support, error recovery quality, or grammar
availability. Sticking with the C library via cgo is the honest
choice for a v1 that wants tree-sitter-quality output.

**Cgo tradeoff**

hex is currently pure Go. Adding cgo means:

- Slower first-time builds (compile C sources for grammars)
- Cross-compile requires `CC` toolchain for the target
- CI matrices grow (macOS/Linux/Windows all need clang or gcc)
- Static-binary story unchanged (grammars link in fine)

Mitigation: the tree-sitter packages are **opt-in submodules of
hex's module tree** but importable independently. Consumers who
don't `import "github.com/jordanbrauer/hex/tree-sitter/lua"`
don't compile any C.

If pain grows, we can further split `hex/tree-sitter` into its own
Go module so `go get github.com/jordanbrauer/hex/...` stays cgo-free.
That's a follow-up if needed.

### API sketch

```go
package treesitter

import (
    ts "github.com/smacker/go-tree-sitter"
)

// Language is a compiled grammar plus its highlight queries.
type Language struct {
    grammar   *ts.Language
    highlight *ts.Query
    name      string
}

// Parser holds a single tree-sitter parser for one language. Reusable
// across parses; not goroutine-safe (callers own serialisation).
type Parser struct {
    ts   *ts.Parser
    lang *Language
}

func NewParser(lang *Language) *Parser
func (p *Parser) Parse(source []byte, previous *Tree) *Tree
func (p *Parser) Close()

// Tree is a parsed AST. Cheap to hold; mutation happens by re-parsing
// with the previous tree passed for incremental updates.
type Tree struct { ... }
func (t *Tree) Root() Node
func (t *Tree) Close()

// HighlightSpan is a byte range in the source and the semantic
// category tree-sitter's query engine assigned to it.
type HighlightSpan struct {
    Start, End int
    Capture    string  // "keyword", "string", "comment", "function", ...
}

// Highlights runs the language's highlight query over the tree and
// returns non-overlapping spans in source order.
func (t *Tree) Highlights(source []byte) []HighlightSpan
```

Language packages export a single constructor:

```go
package lua

import "github.com/jordanbrauer/hex/tree-sitter"

//go:embed highlights.scm
var highlightsSCM string

func Language() *treesitter.Language { ... }
```

### Style mapping

Tree-sitter's highlight-query capture names follow a de-facto
convention (`@keyword`, `@string.escape`, `@function.builtin`, etc.).
`hex/tree-sitter/theme` maps them to `lipgloss.Style` values:

```go
package theme

type Theme struct {
    Styles map[string]lipgloss.Style  // capture name → style
}

func Default() Theme  // hex's built-in palette
func (t Theme) Style(capture string) lipgloss.Style
```

Users override by copying and mutating a Theme, or building one from
scratch.

Rendering a highlighted string:

```go
func Render(source []byte, spans []HighlightSpan, theme Theme) string
```

Emits the source with lipgloss styles applied per span. Non-covered
regions render plain.

### REPL integration

Two touchpoints:

**1. Highlight-on-echo (easier)**

`tui/components/repl` gains an optional `Highlighter` field:

```go
type Options struct {
    ...
    Highlighter func(mode, source string) string  // returns styled source
}
```

Applied when echoing submitted input to scrollback. `hex/lua/repl`
wires it per-mode: Teal source uses the Teal grammar, Lua uses the
Lua grammar.

Rough impl: `treesitter.Render(source, tree.Highlights(source), theme)`.

**2. As-you-type highlighting (harder)**

Needs the input line itself to render with per-range styles. Current
`bubbles/textinput` renders as a single styled string; per-character
styling requires either:

- **Fork textinput** with a Highlighter hook — modest effort, cleanest.
- **Replace textinput with a hex-owned line editor** — bigger, but
  sets up for tab completion popups and inline ghost text.

The second option is where we probably end up regardless. Design of
that line editor is a separate document (see Follow-ups).

### Tree-sitter for other hex surfaces

Highlighting isn't the only use. Same parser can drive:

- **`hex check` output** — highlight the offending line and mark the
  error column
- **Log rendering** — snippet formatter for structured errors that
  include source context
- **Script file introspection** — `hex list-symbols script.tl` for
  quick surveys during a large refactor

None of these are v1 requirements, but the architecture should
accommodate them. `hex/tree-sitter` is language-agnostic; higher
layers pick languages as needed.

## Phased delivery

Each phase is independently useful and shippable.

**Phase 1 — Foundation**

- Add `github.com/smacker/go-tree-sitter` dependency
- `hex/tree-sitter` core package: Parser, Tree, HighlightSpan, Render
- `hex/tree-sitter/lua` grammar package + embedded `highlights.scm`
  (copied from the official tree-sitter-lua repo, kept in-tree)
- `hex/tree-sitter/theme` with default palette
- Unit tests: parse sample Lua source, spans match expected captures
- One demo: `hex check-lua path.lua` prints highlighted source

**Phase 2 — Highlight-on-echo REPL**

- Add `Highlighter` field to `tui/components/repl.Options`
- `hex/lua/repl.runInteractive` wires the highlighter per mode
- Both `.lua` and `.tl` echoes render styled in scrollback
- User visible from a fresh `hex init` → `myapp repl`

**Phase 3 — Teal grammar**

- `hex/tree-sitter/teal` package
- Import community grammar (with attribution / license note)
- Wire into REPL's Teal-mode highlighter

**Phase 4 — Custom line editor**

- New package `hex/tui/components/lineeditor` (or absorb into repl)
- Replaces `bubbles/textinput` in `hex/tui/components/repl`
- Supports per-range styling → as-you-type highlighting
- Groundwork for completion popup + ghost text

**Phase 5 — Tab completion**

- `hex/tree-sitter` node helpers: NodeAt(cursorPos), EnclosingScope,
  PreviousToken
- Completion source combines tree-sitter context + Teal's type stubs
  (already have those from Phase 3.5)
- `db.<TAB>` → `query queryOne exec transaction`

**Phase 6 — Structural queries for consumers**

- Documented API for running arbitrary tree-sitter queries
- Enables consumer tools: symbol search, refactor scripts, etc.

Ordering isn't strict. Phase 2 could ship before Phase 3 (Lua-only
highlight while Teal falls back to plain). Phase 4 is the biggest
single lift.

## Alternatives considered

**Chroma (`github.com/alecthomas/chroma`)**

- Pure Go, no cgo
- Ships Lua lexer built in
- Static tokenising — no incremental parse, no error recovery,
  no query engine
- No Teal support (we'd write a lexer or fall back to Lua)

Excellent for highlight-on-echo alone. Doesn't scale to tab completion
or structural queries because there's no AST to walk.

If we decided tree-sitter's cgo cost was unacceptable, chroma is the
fallback. This doc assumes we accept cgo.

**Hand-rolled lexers**

Every language a bespoke lexer. Fast to ship one, endless work to
maintain and extend. Rejected on principle — tree-sitter's ecosystem
is exactly what we want to reuse.

**Language server (LSP)**

Full LSP overhead for a REPL is wildly disproportionate. Might revisit
for `hex/lsp` if we ever ship editor tooling for hex projects. Even
then, LSP servers often use tree-sitter internally.

## Open questions

1. **Grammars in-tree or vendored via go.mod?**
   smacker/go-tree-sitter bundles many grammars. Do we depend on their
   Lua bindings or ship our own to control grammar versioning + query
   files?

2. **Teal grammar provenance and maintenance?**
   `euclidianAce/tree-sitter-teal` is community. We may need to fork /
   pin. Long-term, contributing back is the right move.

3. **How to handle Teal-in-Lua mode?**
   The REPL supports mode switching. A Teal-typed global used in Lua
   mode: what parser highlights it? Simplest answer: the mode at
   submit time. Slightly wrong when a Lua statement references a
   Teal-declared identifier, but acceptable.

4. **Cross-compilation story?**
   Many hex users cross-compile to Linux from Mac all the
   time. Cgo-with-C-grammars complicates this. Do we ship a
   pure-Go fallback path that disables highlighting on cross-compile
   targets without a cgo toolchain?

5. **Should the tree-sitter package live in its own Go module?**
   Would make the cgo taint fully opt-in via `go get`. Downside: a
   separate release cadence, another `go.mod`. Probably yes if adoption
   pain surfaces; wait-and-see for now.

## References

- Tree-sitter overview — https://tree-sitter.github.io/tree-sitter/
- go-tree-sitter — https://github.com/smacker/go-tree-sitter
- tree-sitter-lua — https://github.com/tree-sitter/tree-sitter-lua
- tree-sitter-teal — https://github.com/euclidianAce/tree-sitter-teal
- Highlight query syntax — https://tree-sitter.github.io/tree-sitter/syntax-highlighting
- chroma (alternative) — https://github.com/alecthomas/chroma
- Related: [docs/repl.md](../repl.md), [docs/adr/0007-lua-runtime-only.md](../adr/0007-lua-runtime-only.md)
