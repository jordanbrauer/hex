<p align="center">
    <a href="https://github.com/jordanbrauer/hex/actions/workflows/ci.yml"><img alt="ci" src="https://github.com/jordanbrauer/hex/actions/workflows/ci.yml/badge.svg"></a>
    <a href="https://pkg.go.dev/github.com/jordanbrauer/hex"><img alt="godoc" src="https://pkg.go.dev/badge/github.com/jordanbrauer/hex.svg"></a>
    <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
</p>

# hex

**An opinionated Go application framework.**

hex is an IoC container, service providers with a proper lifecycle, a
typed event bus, layered config, structured logging, an HTTP server, a
view engine, an embedded Lua runtime, a queue, a scheduler, a policy
engine, feature flags, i18n, and telemetry — behind coherent interfaces
that compose. A scaffolding CLI generates full projects and individual
pieces (providers, controllers, migrations, commands) following the same
conventions the framework enforces at runtime.

Write your business logic. Let hex handle the rest.

## Install

**Homebrew (macOS + Linux, recommended):**

```sh
brew tap jordanbrauer/hex https://github.com/jordanbrauer/hex
brew install jordanbrauer/hex/hex
```

**Go toolchain:**

```sh
go install github.com/jordanbrauer/hex/cmd/hex@latest
```

Scaffold a new project:

```sh
hex init myproject --db sqlite --web
cd myproject
go run . serve
```

Point a browser at <http://localhost:8080>.

## What's in the box

| Concern | Package |
|---|---|
| App kernel + bootstrap | [`hex`](./) |
| IoC container | [`container`](./container) |
| Service providers | [`provider`](./provider) |
| Typed event bus | [`events`](./events) |
| Layered config (TOML + CUE + env) | [`config`](./config) |
| Database + migrations | [`db`](./db), [`db/sqlite`](./db/sqlite), [`db/postgres`](./db/postgres) |
| Structured logging | [`log`](./log) |
| Cobra CLI scaffolding | [`cli`](./cli) |
| HTTP server + middleware | [`web`](./web) |
| View engine (Go tmpl / Markdown / Jade) | [`view`](./view), [`view/md`](./view/md), [`view/jade`](./view/jade) |
| Embedded Lua (Lua / Teal / Fennel) + REPL | [`lua`](./lua), [`lua/teal`](./lua/teal), [`lua/fennel`](./lua/fennel), [`lua/repl`](./lua/repl) |
| Cache (memory, extensible) | [`cache`](./cache), [`cache/memory`](./cache/memory) |
| Queue + jobs (memory, sqlite) | [`queue`](./queue), [`queue/memory`](./queue/memory), [`queue/sqlite`](./queue/sqlite), [`queue/jobs`](./queue/jobs) |
| Cron scheduler | [`cron`](./cron) |
| Multi-backend filesystem | [`disk`](./disk), [`disk/local`](./disk/local) |
| Worker pool | [`pool`](./pool) |
| Authorisation policy | [`policy`](./policy) |
| Internationalisation | [`i18n`](./i18n) |
| Feature flags | [`featureflag`](./featureflag) |
| OpenTelemetry | [`telemetry`](./telemetry) |
| BDD / Gherkin testing | [`bdd`](./bdd) |
| Web-app testing (HTTP + DOM) | [`webtest`](./webtest), [`webtest/bdd`](./webtest/bdd) |
| TUI primitives | [`tui`](./tui) |
| Small utilities | [`clock`](./clock), [`id`](./id), [`errors`](./errors), [`hash`](./hash), [`retry`](./retry), [`ratelimit`](./ratelimit), [`httpx`](./httpx), [`validate`](./validate), [`env`](./env), [`build`](./build) |
| Scaffolder CLI | [`cmd/hex`](./cmd/hex) |

## A quick tour

**Bootstrap an app.**

```go
package main

import (
    "context"
    "os"

    "github.com/jordanbrauer/hex"
    hexcli "github.com/jordanbrauer/hex/cli"
    hexlog "github.com/jordanbrauer/hex/log"

    "myproject/app"
    "myproject/app/command"
)

func main() {
    hexlog.Init()

    kernel := hex.New()
    if err := app.Boot(kernel); err != nil {
        hexlog.Fatal("register providers", "error", err)
    }

    ctx := context.Background()
    if err := kernel.Bootstrap(ctx); err != nil {
        hexlog.Fatal("bootstrap", "error", err)
    }
    defer func() { _ = kernel.Shutdown(ctx) }()

    os.Exit(hexcli.Execute(command.Root(kernel)))
}
```

**Write a service provider.**

```go
type Users struct{ provider.Base }

func (p *Users) Register(app provider.Application) error {
    app.Singleton("users", func(c *container.Container) (any, error) {
        db, err := container.Make[*sql.DB](c, "db")
        if err != nil {
            return nil, err
        }
        return NewUserService(db), nil
    })
    return nil
}
```

**Test a route end-to-end.**

```go
client := webtest.New(t, app)

client.Get("/dashboard").
    StatusIs(200).
    See("Welcome, Alice").
    Find(".user-card").HasClass("active").
    Find("button").Count(3)
```

## Examples

Runnable example apps under [`examples/`](./examples):

- [`examples/swapi`](./examples/swapi) — a Star Wars API demo, scaffolded
  via `hex init`, serving the classic SWAPI dataset out of a SQLite file.
- [`examples/ai-lua`](./examples/ai-lua) — a minimal app that boots
  `hex/ai` + `hex/lua` and executes Lua scripts that call an LLM through
  the `agent` module.

## Documentation

- [`docs/PLAN.md`](./docs/PLAN.md) — the framework's scope and package
  roster.
- [`docs/adr/`](./docs/adr) — architecture decisions, numbered and
  written in past tense. Read the ADR covering the area you're touching
  before changing it.
- [`docs/repl.md`](./docs/repl.md) — user-facing REPL reference.
- [`docs/designs/`](./docs/designs) — design docs for work-in-flight
  proposals.
- [`CONTEXT.md`](./CONTEXT.md) — hex-specific vocabulary (App, Container,
  Binding, Provider, Bootstrap, Disk, Cache, Job, Pool, …). Use this
  vocabulary in code and prose.

## Requirements

- Go 1.26 or newer.
- No cgo requirements in the base install. SQLite uses
  `modernc.org/sqlite` (pure Go), so cross-compilation from macOS to
  Linux "just works".

## Contributing

Pull requests welcome. Read [`CONTRIBUTING.md`](./CONTRIBUTING.md) for
the process and [`AGENTS.md`](./AGENTS.md) if you're an AI coding agent
touching the codebase.

Every PR must pass `just check` — `gofmt`, `go vet`, `golangci-lint`,
and the race-enabled test suite. CI mirrors that gate.

## Status

hex is pre-1.0. The core packages are stable in shape; minor breaking
changes may still land ahead of a 1.0 tag. Watch the repository or
subscribe to release notifications if you're building on top of it.

## Inspiration & references

hex stands on the shoulders of many prior frameworks and libraries.
Direct influences:

- **[Laravel](https://laravel.com)** — the service container, service
  provider lifecycle, and artisan-style scaffolder.
- **[Phoenix](https://phoenixframework.org)** — the runtime environment
  as a first-class concept, the layered supervisor tree feel of the
  provider registry, and channel-flavoured event bus semantics.
- **[Ruby on Rails](https://rubyonrails.org)** — convention over
  configuration, generators, and the "one canonical place for each
  concern" project layout.
- **[Hugo](https://gohugo.io)** — proof that Go can carry an opinionated
  framework with a great CLI.
- **[Algernon](https://algernon.roboticoverlords.org)** — a Go web server
  that embeds gopher-lua alongside markdown and templates, and did the
  gopher-lua ↔ Go-modules dance long before hex.

Notable Go libraries hex builds on:

- [`labstack/echo`](https://github.com/labstack/echo) — HTTP routing and
  middleware.
- [`spf13/cobra`](https://github.com/spf13/cobra) and
  [`spf13/viper`](https://github.com/spf13/viper) — CLI and config.
- [`charmbracelet/log`](https://github.com/charmbracelet/log) and the
  wider [charm](https://charm.sh) TUI ecosystem.
- [`yuin/gopher-lua`](https://github.com/yuin/gopher-lua) — the embedded
  Lua VM, plus [Teal](https://github.com/teal-language/tl) and
  [Fennel](https://fennel-lang.org) on top of it.
- [`casbin/casbin`](https://github.com/casbin/casbin) — the authorisation
  model.
- [`golang-migrate/migrate`](https://github.com/golang-migrate/migrate)
  — database migrations.
- [`cuelang.org/go`](https://cuelang.org) — config schema validation.

Full list of dependencies and the ADR that motivated each wrapping
decision is in [`docs/adr/`](./docs/adr).

## License

MIT — see [LICENSE](./LICENSE).
