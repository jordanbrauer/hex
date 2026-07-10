# Instructions for coding agents

This file is a compressed reference for AI coding agents (Claude, Cursor,
Aider, and friends) working in the hex repo. Human contributors should read
`CONTRIBUTING.md`; the two documents overlap but this one is the fast path
for machines.

## What hex is

An opinionated Go application framework — like Laravel, Phoenix, or Hugo —
plus a scaffolding CLI (`cmd/hex`). Everything users need to build a real
Go service lives in this repo: DI container, service providers, event bus,
config, logging, database, HTTP server, view engine, Lua runtime, and so
on. A separate `hex` template repository (analogous to `laravel/laravel`)
will exist for consumer applications; **this repo is `laravel/framework`**,
not the app template.

## Read before you edit

- `CONTEXT.md` — glossary of hex-specific terms (App, Container, Binding,
  Provider, Bootstrap, Disk, Cache, Job, Pool, …). Match this vocabulary
  in code, tests, docs, and commit messages. Never invent a synonym.
- `docs/PLAN.md` — the framework's scope, package roster, phase plan.
  Consult this before adding a new package.
- `docs/adr/` — architecture decisions that are locked in. Read the ADR
  covering the area you're touching before changing it. If your change
  invalidates an ADR, propose a new ADR to supersede it in the same PR.
- `docs/repl.md` — user-facing REPL reference; keep in sync with
  `lua/repl/` changes.

## Repo layout

```
/                       # one Go module: github.com/jordanbrauer/hex
├── <package>/          # top-level packages: container, provider, events,
│                       #   config, db, log, cli, cron, cache, disk, tui,
│                       #   web, lua, queue, pool, policy, i18n,
│                       #   featureflag, clock, id, errors, hash, retry,
│                       #   ratelimit, httpx, validate, telemetry, bdd,
│                       #   view, webtest, env, build, hextest
├── <package>/provider/ # optional — service provider that wires the
│                       #   package into hex.App. Consumers register this
│                       #   in their app/boot.go.
├── <package>/lua/      # optional — Lua bindings for the package
├── <package>/lua/provider/  # optional — provider that installs the Lua
│                             #   module into the shared runtime
├── <package>/<driver>/ # subpackages for driver / backend impls,
│                       #   e.g. db/{sqlite,postgres}, cache/memory,
│                       #   disk/local, queue/{memory,sqlite}
├── cmd/hex/            # the scaffolding CLI
├── examples/           # runnable example apps (ai-lua, swapi, …)
└── docs/               # PLAN, ADRs, designs, reference docs
```

## Conventions

### Wrap known-good libraries, don't reinvent

Every `hex/<pkg>` that wraps a well-known library is deliberate and
documented in an ADR:

| Package | Wraps | ADR |
|---|---|---|
| `hex/cron` | `robfig/cron` | — |
| `hex/log` | `charmbracelet/log` | — |
| `hex/web` | `labstack/echo` | 0006 |
| `hex/lua` | `yuin/gopher-lua` | 0007 |
| `hex/queue` | (bespoke; sqlite via `mattn/go-sqlite3`) | 0009 |
| `hex/pool` | `alitto/pond` | 0010 |
| `hex/policy` | `casbin/casbin` | 0011 |
| `hex/i18n` | `leonelquinteros/gotext` | 0012 |
| `hex/featureflag` | `thomaspoignant/go-feature-flag` | 0013 |
| `hex/telemetry` | OpenTelemetry SDK | 0014 |
| `hex/bdd` | `go-bdd/gobdd` | 0015 |
| `hex/view/md` | `yuin/goldmark` | — |
| `hex/view/jade` | `Joker/jade` | — |
| `hex/lua/fennel` | vendored `fennel-1.6.1.lua` | — |

Wrappers stay thin. Escape hatches to the underlying type are expected
(`web.Server.Echo()`, `db.DB()`). Do not fork; do not vendor unless the
upstream is unmaintained and pinned (only `fennel-1.6.1.lua` qualifies
today, with a NOTICE.md).

### No import aliases

Go's namespaces handle same-name imports without aliasing. Only use
aliases when the compiler forces you to disambiguate two `hex` packages
imported into the same file. Consumers of the scaffolder never see
aliases in generated code — templates enforce this.

### Test data as real files under `testdata/`, embedded via `//go:embed`

Never inline schema, `.feature`, `.cue`, `.po`, `.toml`, or template
fixtures as Go string literals. Put them in `testdata/`, embed the
directory, and read via `fs.FS`. This keeps fixtures diff-friendly,
syntax-highlighted, and independently editable.

### Test naming

`TestX_behaviourItGuarantees` — the underscore-separated suffix
describes the *guarantee under test*, not the setup. Subtests use
`t.Run("thing it does", …)` with sentence-case names.

### Provider lifecycle: Register → Boot → Shutdown

- **Register**: bind singletons into the container. Cheap, no I/O, no
  goroutines. Providers that install Lua modules do it here.
- **Boot**: open resources, run migrations, start goroutines. Any
  expensive work goes here. Order matches registration order.
- **Shutdown**: reverse-order cleanup. Only providers that need it
  implement `Shutdown`.

Do not call `container.Make` inside `Register()` — the value you need
may not be bound yet, and singleton errors get cached (see
`hex/container` docs). If you need a value in Register, defer it: bind a
closure that resolves the dependency lazily on first call.

### Scaffolder marker comments

The `cmd/hex/make:*` generators insert new registrations above magic
comments in the scaffolded app's tree:

- `// hex:providers` in `app/boot.go`
- `// hex:commands` in `app/command/root.go`
- `// hex:routes` in `app/provider/routes.go`
- `// hex:repl` in `app/provider/repl_bindings.go`

The marker must be the first non-whitespace token on its line — Go
docstrings do not qualify. If you edit a scaffolder template, keep the
marker on its own line, unindented from the pattern the generator
expects.

## Building and testing

```
just build          # go build ./...
just test           # go test ./...
just race           # go test -race ./...
just cover          # HTML coverage report
just fmt            # gofmt -s -w .
just vet            # go vet ./...
just lint           # fmt-check + man-check (manpage drift) + golangci-lint
just check          # lint + vet + race — the pre-commit gate
just tidy           # go mod tidy
just clean          # remove coverage + testcache
```

`fmt-check` and `man-check` are `[private]` recipes — helpers folded
into `lint`, not meant to be run standalone in normal workflow (though
`just fmt-check` / `just man-check` still work directly).

Every PR must pass `just check` before merging. CI mirrors this gate
across four jobs: `qa` (gofmt + vet + golangci-lint) and `docs`
(manpage markdown must match the command tree) run in parallel with
`build`; `test` (race-enabled) only runs if `qa` and `build` both pass.

## Known flaky tests

None at the moment. If you introduce or discover a flake, document it
here along with the reproducer command so future contributors don't
spin their wheels on it.

## Dependency policy

- New direct dependencies require justification in the PR description
  or a follow-up ADR.
- Wrapping a well-known library is strongly preferred over hand-rolling.
- Prefer pure-Go libraries. Cgo is acceptable when there's no viable
  alternative (`modernc.org/sqlite` is our go-to over `mattn/go-sqlite3`
  for cross-compilation reasons).
- Deprecated / unmaintained libraries get vendored with a NOTICE.md, not
  imported.

## Docs discipline

Every meaningful package has a top-of-file package comment that answers
"what is this and when do you use it?". Every exported symbol has a
docstring that reads as full sentences with punctuation. Every ADR:

- Numbered sequentially, filename `NNNN-kebab-case-title.md`
- Sections: **Decision**, **Context**, **Consequences**
- Written in past tense — an ADR records a decision that's been made,
  not a proposal

## Commit and PR conventions

See `CONTRIBUTING.md` — one-line summary: imperative mood, present
tense, no conventional-commits prefix required, reference issues with
`Closes #N` on their own line at the end of the body.
