# Contributing to hex

Thanks for wanting to contribute. This document is the human-facing
counterpart to [`AGENTS.md`](./AGENTS.md) (which is the same content
compressed for AI coding agents).

## Before you start

1. Read [`CONTEXT.md`](./CONTEXT.md) — hex has a strict vocabulary for
   its core concepts (App, Container, Binding, Provider, Bootstrap,
   Disk, Cache, Job, Pool, …). Match it in code, tests, docs, and
   commit messages.
2. Skim [`docs/PLAN.md`](./docs/PLAN.md) for the scope and package
   roster.
3. If your change touches an area covered by an [ADR](./docs/adr), read
   the relevant ADR first. If your change invalidates one, propose a
   new ADR that supersedes it in the same PR.

## Development setup

```sh
git clone https://github.com/jordanbrauer/hex
cd hex

# Verify everything is wired.
just check
```

Common recipes:

```sh
just build          # go build ./...
just test           # go test ./...
just race           # go test -race ./...
just cover          # HTML coverage report at coverage.html
just fmt            # gofmt -s -w .
just fmt-check      # fail if anything would change
just vet            # go vet ./...
just lint           # golangci-lint (see .golangci.yml)
just check          # fmt-check + lint + vet + race — the pre-commit gate
just tidy           # go mod tidy
```

Every PR must pass `just check`. CI runs the same gate: `qa` (gofmt +
vet + golangci-lint) and `build` in parallel, then `test` (race-enabled)
after both succeed.

## Requirements

- Go 1.26 or newer.
- [`just`](https://github.com/casey/just) if you want the recipe
  convenience; the underlying commands are plain `go`.
- [`golangci-lint`](https://golangci-lint.run) for `just lint`.

## What to work on

Open issues tagged `good first issue` are safe starting points. If you
want to build something bigger, open a discussion issue first so the
scope can be agreed before code moves.

Areas that welcome contributions:

- **New drivers** for existing packages (e.g. `disk/s3`, `disk/gcs`,
  `queue/sqs`, `cache/redis`, `db/mysql`). Follow the pattern of the
  existing driver in the same package.
- **View preprocessors** for `hex/view` (see `view/md` and `view/jade`).
- **Framework Lua modules** (see the pattern under `db/lua`, `cache/lua`,
  and friends).
- **Documentation** — better package docstrings, tutorial content
  under `docs/`, more runnable examples under `examples/`.
- **Tests** — coverage gaps, new webtest / bdd scenarios for existing
  behaviour.

## Commit style

Use imperative mood in the subject: **"add X"**, **"fix Y"**,
**"rename Z"** — describe what the commit does, not what it did.

Keep the subject under ~72 characters. Use a blank line before the
body. Explain the *why* in the body, not the *what* (the diff already
tells you what changed).

Reference issues at the end of the body:

```
add cache/redis driver

Wires a Redis-backed Cache implementation using go-redis/v9. Follows
the same shape as cache/memory: byte + generic surfaces, TTL semantics,
container binding via cache/provider with driver = "redis".

Closes #42
```

There's no required `feat:` / `fix:` / etc. prefix — the ticket
reference at the bottom does that job, and prefixes add noise when
scanning `git log`.

## Pull requests

- **Small, single-purpose PRs.** If your branch does two things, split
  it. Reviewers can approve small PRs faster and catch more.
- **Descriptions matter.** Explain the motivation, the design choice,
  and any alternatives you rejected. If your change is a bug fix,
  include steps to reproduce the bug.
- **Every PR must pass `just check`** before it can be merged. CI runs
  the same gate.
- **Include tests** for behaviour changes. Fixtures live under
  `testdata/` and get embedded via `//go:embed` — never inline them as
  Go string literals.
- **Update documentation** in the same PR. If you add or change public
  API, update the package doc comment. If you touch REPL behaviour,
  update `docs/repl.md`. If you make a load-bearing decision, add an
  ADR under `docs/adr/`.

## Code conventions

Full detail lives in [`AGENTS.md`](./AGENTS.md). The most-cited ones:

- **Wrap known-good libraries** rather than roll your own. Every
  wrapper is documented in an ADR.
- **No import aliases** unless the compiler forces you to disambiguate.
- **Testdata as real files**, not string literals. Embed with
  `//go:embed`.
- **Test names** are `TestX_behaviourItGuarantees` — the suffix
  describes the *guarantee*, not the setup.
- **Provider lifecycle**: `Register` binds cheap; `Boot` opens
  resources; `Shutdown` closes them in reverse. Do not `container.Make`
  inside `Register` — defer expensive resolution into a factory closure.
- **Docstrings are full sentences** with punctuation. Every exported
  symbol has one. Every package has a top-of-file comment answering
  "what is this and when do you use it?".

## Adding a new package

1. Open a discussion or issue first — is it in scope for hex?
2. If yes, write the ADR under `docs/adr/NNNN-kebab-case-title.md`
   (past tense: it records a decision that's been made).
3. Add the package to the "In scope" table in `docs/PLAN.md`.
4. Add the package to the wrapped-libraries table in `AGENTS.md` if it
   wraps something.
5. Ship the package + its `provider/` subpackage + tests + doc comment.
6. Link it in the top-level `README.md` package matrix.

## Known flaky tests

Two tests are known-flaky under `-race`. Do not treat them as blocking
unless you're specifically debugging them:

- `queue/memory.TestCompetingConsumers_splitStream`
- `web.TestUserRouteWorks`

If you fix either, note it in the PR and remove it from this list (and
from the equivalent list in `AGENTS.md`).

## Code of conduct

Be kind. Assume good faith. Argue about ideas, not people. If you feel
uncomfortable, say so — the project maintainer will listen.

## License

By submitting a pull request, you agree that your contributions will be
licensed under the [MIT License](./LICENSE) that covers the project.
