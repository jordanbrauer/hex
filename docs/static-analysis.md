# Static analysis

What hex runs today, what's available in the Go ecosystem beyond that,
and a prioritized plan for closing the gaps. Written up after an
audit of the repo's meta/tooling surface (see git log around
`.github/dependabot.yml`, `SECURITY.md`, etc. for the sibling audit).

## What hex runs today

`.golangci.yml` deliberately runs a **lenient** 3-linter set:

- `govet` — official, compiler-adjacent bug detection
- `staticcheck` — SA-rules: real bugs, dead code, performance
- `ineffassign` — unused assignments

This is intentional (see the comment at the top of `.golangci.yml`):
enough to catch real bugs without blocking existing code on style,
ratcheted up deliberately rather than all at once. `just lint` also
folds in `fmt-check` (gofmt) and `man-check` (generated manpage
markdown drift) — see `docs/release.md` and the justfile.

CI mirrors this via the `qa` job in `.github/workflows/ci.yml`.

## The Go static analysis landscape

Useful framing: Go's compiler already does most of what tools like
PHPStan/Psalm exist to bolt onto PHP — real static types, no
null-vs-undefined ambiguity, no dynamic property bags, exhaustive
type-checking at build time. The "missing layer" those tools fill in
PHP is much thinner in Go. What's left splits into distinct concerns.
`golangci-lint` (already hex's setup) is the aggregator that bundles
all of them behind one config — like running PHPStan + Psalm +
PHP-CS-Fixer + Rector all through one runner.

### Bug-pattern detection (closest thing to PHPStan's "levels")

| Tool | What it catches | In hex today? |
|---|---|---|
| `staticcheck` | Real bugs, dead code, perf | ✅ |
| `govet` | Compiler-adjacent mistakes (struct copy w/ lock, printf arg mismatch, unreachable code) | ✅ |
| `gosec` | Hardcoded creds, weak crypto, SQL injection via string concat, unsafe file perms | ❌ |
| `errcheck` | Unchecked error returns | ❌ |

### Complexity metrics

| Tool | Measures |
|---|---|
| `gocyclo` | Classic McCabe cyclomatic complexity per function |
| `gocognit` | Cognitive complexity — weights nesting more than raw branch count; usually a better signal than cyclomatic |
| `cyclop` | Package + function complexity, newer alternative to gocyclo |
| `maintidx` | Maintainability index (complexity + volume + comments, Halstead-derived) |
| `funlen` | Raw function length (lines/statements) |
| `nestif` | Flags deeply nested `if` chains specifically |

None of these are enabled today. A one-off run against the repo
(`gocyclo`/`gocognit`/`maintidx`/`funlen`, thresholds 12/15/-/80 lines)
surfaced, notably:

- `tui/markup.go:parse` — cognitive complexity **109** (extreme outlier)
- `tui/components/repl.go:(Model).Update` — **51**
- `config/config.go:Load` — **32**
- `cmd/hex/app/command/init.go:scaffold` — **30**
- Several more in the high-teens/20s across `cmd/hex/app/command/init.go`,
  `config/`, `lua/repl/`, `httpx/`

This isn't a judgment that those functions are wrong — some (a large
`Update` switch, a markup parser) are inherently branchy — but it's a
concrete list to start from if/when the linter set gets stricter.

### Style/idiom enforcement (PHP-CS-Fixer / Psalm's stricter checks equivalent)

`revive` (modern `golint` replacement), `gocritic` (style + minor bug
diagnostics, has auto-fix), `ireturn`, `unparam`, `unconvert`,
`wastedassign`. None enabled today.

### Duplication

`dupl` — literal code-clone detection. No real Go equivalent existed
before this; PHP has similar via PHPMD's copy-paste detector. Not
enabled today.

### Dependency vulnerability scanning

`govulncheck` — the official Go team tool. This is the actual
equivalent of `composer audit` / PHP Security Advisories DB, and it's
meaningfully better than most ecosystems' versions of this check: it
cross-references your dependencies against the Go vulnerability
database **and your actual call graph**, so it only flags vulns in
code paths you actually reach, not just "a vuln exists somewhere in
this dependency tree." Cheap, fast, zero config. Not wired up today.

## CodeQL

CodeQL is GitHub's semantic code-analysis engine. Distinct from
everything above: instead of pattern/AST matching, it compiles the
code (via `go build`, traced) into a relational database representing
data flow, call graphs, and control flow, then runs dataflow queries
against it — e.g. "does user input reach `os/exec.Command` without
sanitization," SQL injection via `db/sql`, SSRF via unvalidated URLs.
This catches taint/injection bugs that pattern-based linters
(`gosec` included) only approximate.

**Runs two ways, not mutually exclusive:**

- **CI** (`.github/workflows/codeql.yml`, not yet added): on push/PR to
  `main` + a weekly cron. Findings land in the repo's Security → Code
  scanning tab.
- **Local CLI**, fully offline after the initial download (no GitHub
  account/token required to run queries):

  ```sh
  # one-time: download the CLI + bundled query packs
  gh extension install github/gh-codeql   # or grab the codeql-bundle release directly

  # create a database — this runs `go build` under a tracer
  codeql database create hexdb --language=go --source-root=.

  # run the standard Go query pack
  codeql database analyze hexdb codeql/go-queries --format=sarif-latest --output=results.sarif
  ```

  View results via the VS Code CodeQL extension (renders sarif
  interactively) or convert to text/csv. Not installed on any dev
  machine as of this writing.

**Why it's relevant to hex specifically, beyond generic due diligence:**
hex embeds a **Lua runtime with a REPL** (arbitrary code execution
surface — gopher-lua sandbox correctness matters) and `hex init`
**generates files onto disk from user-controlled input** (path
traversal / arbitrary file overwrite is a real category here, not
theoretical). `hex/policy` (authz), `hex/httpx`/`hex/web` (SSRF/
injection in wrapped echo handlers) are the other areas most likely to
benefit from dataflow analysis over pattern matching.

## Plan

Ordered by cost/value, not urgency — nothing here is a fire.

| # | Action | Why this order | Status |
|---|---|---|---|
| 1 | Add `govulncheck` as a `just` recipe + CI step | Cheap, zero config, orthogonal to everything else, highest signal-to-noise of anything on this list | Not started |
| 2 | Add `.github/workflows/codeql.yml` (default Go query pack, on-push + weekly cron) | Free for public repos; catches a bug class (taint/injection) nothing else here covers; most relevant given the Lua + file-generation surface | Not started |
| 3 | Revisit `.golangci.yml`'s lenient set — candidates: `gosec`, `errcheck` | Bug-pattern detection, not style; low false-positive risk relative to the style/complexity tools | Deferred — matches `.golangci.yml`'s documented "ratchet up in follow-up passes" stance |
| 4 | Complexity linters (`gocognit`, `funlen`, `maintidx`) | Style/maintainability, not correctness; real hits exist (see list above) but no urgency; would need per-function `//nolint` triage for the outliers before enabling repo-wide | Deferred |
| 5 | Duplication (`dupl`) + broader idiom set (`revive`, `gocritic`) | Highest false-positive/bikeshed risk of the group; land last if at all | Deferred |

Each row that gets picked up should follow the same pattern as the
existing `.golangci.yml`: document *why* a check is included or
excluded inline, the way the current file explains its `SA1019`/
`QF1001`/`fieldalignment` exclusions.
