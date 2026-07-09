<p align="center">
    <em>An opinionated Go application framework.</em>
</p>

<p align="center">
    <a href="https://github.com/jordanbrauer/hex/actions/workflows/ci.yml"><img alt="ci" src="https://github.com/jordanbrauer/hex/actions/workflows/ci.yml/badge.svg"></a>
    <a href="https://pkg.go.dev/github.com/jordanbrauer/hex"><img alt="godoc" src="https://pkg.go.dev/badge/github.com/jordanbrauer/hex.svg"></a>
    <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-blue.svg"></a>
</p>

# hex

**hex** is a Go application framework for building CLIs, long-running
services, and everything in between. It provides an IoC container, service
providers with a proper lifecycle, a typed event bus, config layering,
structured logging, database plumbing, an HTTP server, a view engine, an
embedded Lua runtime, and a scaffolding CLI that stitches it all together
into idiomatic, opinionated projects.

If you've enjoyed Laravel, Phoenix, or Hugo, hex will feel familiar. If
you've mostly written Go, hex will feel like the missing "batteries" —
the things every real service ends up needing that the standard library
doesn't provide.

> **This repository is the framework**, analogous to
> [`laravel/framework`](https://github.com/laravel/framework). The template
> repository that consumer applications start from (analogous to
> [`laravel/laravel`](https://github.com/laravel/laravel)) is scaffolded
> by the `hex init` command in this repo.

## Install

The scaffolder installs like any other Go binary:

```sh
go install github.com/jordanbrauer/hex/cmd/hex@latest
```

Then in a new directory:

```sh
hex init myapp --db sqlite --web
cd myapp
go run . serve
```

Point a browser at <http://localhost:8080>.

## What ships in the box

| Concern | Package | Notes |
|---|---|---|
| App kernel | [`hex`](./) | Boot orchestration, container access, environment awareness |
| DI | [`container`](./container) | Bind, Singleton, Make with type-safe generics |
| Providers | [`provider`](./provider) | Register → Boot → Shutdown lifecycle |
| Events | [`events`](./events) | Typed pub/sub bus |
| Config | [`config`](./config) | Multi-source TOML + CUE + env.yaml, layered by priority |
| Database | [`db`](./db) + [`db/sqlite`](./db/sqlite), [`db/postgres`](./db/postgres) | Driver-agnostic + migrations |
| Logging | [`log`](./log) | `charmbracelet/log` wrapped with hex conventions |
| CLI | [`cli`](./cli) | Cobra root + common flags + version command |
| HTTP | [`web`](./web) + [`web/provider`](./web/provider) | `labstack/echo` + std middleware + health checks |
| Views | [`view`](./view) + [`view/{md,jade}`](./view) | Go html/template + Markdown + Jade preprocessors |
| Lua | [`lua`](./lua) + [`lua/{teal,fennel,repl}`](./lua) | Multi-language Lua runtime + REPL + type stubs |
| Cache | [`cache`](./cache) + [`cache/memory`](./cache/memory) | Named backends, byte + generic surfaces |
| Queue | [`queue`](./queue) + [`queue/{memory,sqlite,jobs}`](./queue) | Message queue + retry/DLQ jobs layer |
| Cron | [`cron`](./cron) | Named jobs, panic recovery, structured logging |
| Disk | [`disk`](./disk) + [`disk/local`](./disk/local) | Laravel-style multi-backend filesystem |
| Pool | [`pool`](./pool) | Worker pool over `alitto/pond` |
| Policy | [`policy`](./policy) | Authorisation via Casbin (ACL / RBAC / ABAC) |
| I18n | [`i18n`](./i18n) | GNU gettext via `leonelquinteros/gotext` |
| Feature flags | [`featureflag`](./featureflag) | `thomaspoignant/go-feature-flag` |
| Telemetry | [`telemetry`](./telemetry) | OpenTelemetry tracer + meter + shutdown |
| BDD | [`bdd`](./bdd) | Gherkin `.feature` via `go-bdd/gobdd` |
| Web tests | [`webtest`](./webtest) + [`webtest/bdd`](./webtest/bdd) | supertest / RTL-style HTTP + DOM assertions |
| TUI | [`tui`](./tui) | Renderer, markup, styles, components |
| Utilities | [`clock`](./clock), [`id`](./id), [`errors`](./errors), [`hash`](./hash), [`retry`](./retry), [`ratelimit`](./ratelimit), [`httpx`](./httpx), [`validate`](./validate), [`env`](./env), [`build`](./build) | The small stuff every service needs |
| Scaffolder | [`cmd/hex`](./cmd/hex) | `hex init`, `hex make:*`, `hex run`, `hex repl` |

Every wrapped library is a deliberate choice, documented in the ADRs
under [`docs/adr/`](./docs/adr).

## A tour, in code

**Bootstrap an app.**

```go
package main

import (
    "context"
    "os"

    "github.com/jordanbrauer/hex"
    hexcli "github.com/jordanbrauer/hex/cli"
    hexlog "github.com/jordanbrauer/hex/log"

    "myapp/app"
    "myapp/app/command"
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

- [`examples/swapi`](./examples/swapi) — a Star Wars API demo scaffolded
  via `hex init`, serving the classic SWAPI dataset with `hex/db` +
  `hex/web` + `hex/view` + `hex/webtest`.
- [`examples/ai-lua`](./examples/ai-lua) — a minimal app that boots
  `hex/ai` + `hex/lua` and executes Lua scripts that call an LLM through
  the `agent` module.

## Documentation

- [`docs/PLAN.md`](./docs/PLAN.md) — the framework's scope, package
  roster, and phase plan.
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

hex is pre-1.0. The core packages (container, provider, events, config,
db, log, cli, web, view, lua, queue, cache, cron) are stable in shape;
minor breaking changes may still land ahead of a 1.0 tag. Watch the
repository or subscribe to release notifications if you're building on
top of it.

## License

MIT — see [LICENSE](./LICENSE).
