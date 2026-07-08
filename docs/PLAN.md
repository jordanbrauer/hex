# hex

An opinionated Go application framework — like Laravel, Phoenix, or Hugo — that standardizes the foundational patterns shared across finch-api/cli and finch-api/bot. Fresh rewrite, informed by both codebases.

hex is not just a library you import. It's a **framework with a CLI** that scaffolds new projects, generates boilerplate, and enforces architectural conventions from day one.

## Problem

The CLI and bot independently evolved the same architectural patterns — IoC container, service providers, registry lifecycle, config loading, database setup, build info, logging. The implementations are near-identical but drift over time. Bug fixes in one don't propagate. New projects copy-paste from whichever repo the author opened last.

hex extracts these patterns into a single, opinionated Go module with a companion CLI tool, so both apps (and future ones) share a common foundation and consistent project structure.

## Philosophy

> *Convention over configuration. Generate, don't copy-paste.*

hex follows the same playbook as Laravel (`artisan`), Phoenix (`mix phx.gen`), Hugo (`hugo new`), and Rails (`rails generate`):

1. **`hex init`** scaffolds a complete, runnable project with the right directory structure, config files, Makefile, and wiring — ready to `go run` immediately.
2. **`hex make:*`** generators produce correctly-placed, correctly-wired code for providers, domains, migrations, commands, and adapters — following hex conventions so every project looks the same.
3. **The framework owns the architecture.** You don't decide where providers go or how config loads. hex decides. You write domain logic and CLI commands.

## Scope

### In scope

| Package | What | Status in CLI | Status in Bot |
|---|---|---|---|
| `hex` (root) | App kernel, bootstrap orchestration | `app/app.go` + `app/bootstrap.go` | `bot/bot.go` + `bot/bootstrap.go` |
| `hex/container` | IoC container (Bind, Singleton, Make, Must) | `lib/ioc/container.go` | `lib/ioc/container.go` |
| `hex/provider` | Service provider interface + registry lifecycle | `lib/provider/*.go` | `lib/provider/*.go` |
| `hex/events` | Typed event bus (pub/sub) | ❌ not present | `lib/events/bus.go` |
| `hex/db` | Database connection + migration runner | `db/connection.go` (SQLite) | `db/connection.go` (Postgres) |
| `hex/config` | Config loading (files + env + embedded defaults) | `config/repository.go` (Viper + TOML) | `config/config.go` (Viper + YAML + env) |
| `hex/build` | Version/commit/time via ldflags | `finch/build.go` | `build/info.go` + `build/init.go` |
| `hex/log` | Structured logging setup | `log/log.go` (charmbracelet/log wrapper) | charmbracelet/log (direct) |
| `hex/cli` | Cobra root command scaffolding + common flags | `cli/root.go` | `cli/root.go` |
| `hex/cron` | Scheduled job runner | ❌ not present | `lib/cron/*.go` |
| `hex/cache` | Multi-backend cache (memcached, redis/valkey, memory) | ❌ not present | ❌ not present |
| `hex/disk` | Laravel-style multi-backend filesystem (`local` first; `s3`/`minio`/`gcs` as subpackages) | `util/disk/*.go` | ❌ not present |
| `hex/tui` | Terminal renderer, markup, console, styles | `tui/{styles,markup,renderer,console,components,wizard}` | ❌ not present |
| `hex/web` | HTTP server (echo) with standard middleware + graceful shutdown | ❌ not present | `web/*.go` + `bot/provider/web.go` |
| `hex/lua` | Lua runtime (gopher-lua). No bindings, no plugin system (ADR-0007) | `lib/lua/*.go` | ❌ not present |
| `hex/queue` | Generic message queue interface + Jobs layer (ADR-0009). Backends: memory, sqlite; later sqs/rabbitmq/kafka | ❌ not present | ❌ not present |
| `hex/pool` | Worker pool for bounded in-process concurrency (wraps alitto/pond, ADR-0010) | ❌ not present | ❌ not present |
| `hex/policy` | Authorisation via Casbin — model + adapter, ACL/RBAC/ABAC (ADR-0011). Adapters: memory, file; later sql | ❌ not present | ❌ not present |
| `hex/i18n` | GNU gettext-compatible i18n via gotext + PO files (ADR-0012). Multi-locale Translator + package-level convenience | ❌ not present | ❌ not present |
| `hex/featureflag` | Feature flags via go-feature-flag (ADR-0013). Retrievers: file, embed.FS | ❌ not present | ❌ not present |
| `hex/clock` | Injectable time source for testable code | ❌ not present | ❌ not present |
| `hex/id` | UUID v4/v7 + ULID + KSUID with one consistent surface | ❌ not present | ❌ not present |
| `hex/errors` | Typed errors with codes + HTTP status mapping | ❌ not present | ❌ not present |
| `hex/hash` | Password hashing (argon2id) + HMAC signature helpers | ❌ not present | ❌ not present |
| `hex/retry` | Generic exponential-backoff retry helper | ❌ not present | ❌ not present |
| `hex/ratelimit` | Token-bucket rate limiter (wraps x/time/rate) | ❌ not present | ❌ not present |
| `hex/httpx` | Outbound HTTP client with retries, backoff, timeout, hex/log integration | ❌ not present | ❌ not present |
| `hex/validate` | Struct/request validation via zog (Zod-style API) | ❌ not present | ❌ not present |
| `hex/telemetry` | OpenTelemetry setup (tracer + metrics + log bridge) | ❌ not present | ❌ not present |
| `hex/bdd` | BDD test runner via gobdd; Gherkin `.feature` support + embed.FS (ADR-0015) | ❌ not present | ❌ not present |
| **`cmd/hex`** | **Scaffolding CLI (`hex init`, `hex make:*`)** | ❌ manual setup | ❌ manual setup |

### Out of scope

These stay in consumer apps — they're app-specific, not framework-generic:

- Domain models, repository interfaces, services (each app's business logic)
- Infrastructure adapters (SQLite repos, Postgres repos, HTTP clients)
- CLI subcommands (app-specific Cobra commands)
- TUI components (CLI-specific bubbletea)
- Lua plugin system (CLI-specific)
- Slack/GitHub integrations (bot-specific)
- API client wrappers (CLI-specific Finch SDK usage)

## Architecture

```
                         ┌─────────────────────┐
                         │     hex CLI tool     │
                         │                     │
                         │  hex init myapp     │──── scaffolds ────┐
                         │  hex make:provider  │──── generates ────┤
                         │  hex make:domain    │──── generates ────┤
                         │  hex make:migration │──── generates ────┤
                         │  hex make:command   │──── generates ────┤
                         │  hex make:adapter   │──── generates ────┤
                         └─────────────────────┘                   │
                                                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        consumer app (generated)                     │
│  (finch-cli, finch-bot, or any future Go service)                   │
│                                                                     │
│  main.go                                                            │
│  ├── hex.New()              ← create app kernel                     │
│  ├── app.Register(...)      ← add app-specific providers            │
│  ├── app.Bootstrap(ctx)     ← Register → Boot all providers         │
│  ├── cli.Root("finch", app) ← Cobra root with common flags         │
│  ├── root.Execute()         ← run                                   │
│  └── app.Shutdown(ctx)      ← reverse-order provider shutdown       │
└─────────────────────────────────────────────────────────────────────┘
         │          │           │           │           │
         ▼          ▼           ▼           ▼           ▼
┌──────────┐ ┌──────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
│container │ │ provider │ │ events  │ │   db    │ │ config  │
│          │ │          │ │         │ │         │ │         │
│ Bind()   │ │ Service  │ │ On[T]() │ │ Open()  │ │ Load()  │
│ Single() │ │ Registry │ │ Emit[T]│ │ Migrate │ │ String()│
│ Make[T]()│ │ Reg/Boot │ │ unsub  │ │ SQLite  │ │ Unmrsh()│
│ Must[T]()│ │ Shutdown │ │ async  │ │ Postgres│ │ Viper   │
└──────────┘ └──────────┘ └─────────┘ └─────────┘ └─────────┘
                                                        │
┌──────────┐ ┌──────────┐ ┌─────────┐                   │
│  build   │ │   log    │ │   cli   │                   │
│          │ │          │ │         │                   │
│ Version()│ │ styled   │ │ Root()  │◄──────────────────┘
│ Commit() │ │ levels   │ │ Version │  (--config flag
│ Time()   │ │ parse    │ │ common  │   feeds config)
│ ldflags  │ │ init     │ │ flags   │
└──────────┘ └──────────┘ └─────────┘
```

## hex CLI — Scaffolding Tool

The `hex` binary is a standalone CLI tool (like `laravel`, `hugo`, `mix phx`). Install it once, use it to create and extend hex projects.

```bash
go install github.com/jordanbrauer/hex/cmd/hex@latest
```

### `hex init` — Create a new project

Scaffolds a complete, runnable hex application.

```bash
hex init myapp
# or inside an existing directory:
mkdir myapp && cd myapp && hex init .
```

**Interactive prompts** (with sensible defaults):

| Prompt | Default | Options |
|--------|---------|--------|
| Go module path | `github.com/<user>/<name>` | any valid module path |
| Database dialect | sqlite | sqlite, postgres, none |
| Config format | toml | toml, yaml |
| Binary name | `<name>` | any string |

**Generated structure:**

```
myapp/
├── cmd/
│   └── myapp/
│       └── main.go               # Entry point — hex.New(), bootstrap, cli.Root()
├── provider/
│   ├── boot.go                   # Bootstrap — registers providers in order (same pattern as cli/bot)
│   └── database.go               # Database provider (if dialect chosen)
├── domain/                       # Empty — your business logic goes here
├── infrastructure/               # Empty — your adapters go here
├── cli/
│   └── root.go                   # Cobra root command + version subcommand
├── db/
│   └── migrations/               # Empty dir (if database enabled)
│       └── .gitkeep
├── config/
│   └── defaults/
│       └── app.toml              # Embedded default config
├── build/
│   └── build.go                  # go:generate-friendly ldflags variables
├── .env.example                  # Example env vars
├── go.mod                        # Module with hex dependency
├── Makefile                      # build, test, lint, migrate targets
├── README.md                     # Quick start docs
└── .gitignore
```

After `hex init`:
```bash
cd myapp
go mod tidy
go run ./cmd/myapp
# ✓ Running — boots providers, prints version, exits cleanly
```

### `hex make:provider` — Generate a service provider

```bash
hex make:provider cache
```

Generates `provider/cache.go`:

```go
package provider

import (
    "context"

    "github.com/jordanbrauer/hex/container"
    "github.com/jordanbrauer/hex/provider"
)

type Cache struct {
    provider.Base
}

func (p *Cache) Register(app provider.Application) error {
    // TODO: bind your cache dependencies
    // app.Singleton("cache", func(c *container.Container) (any, error) {
    //     return NewRedisCache(), nil
    // })
    return nil
}

func (p *Cache) Boot(ctx context.Context, app provider.Application) error {
    return nil
}
```

Also **auto-registers** the provider in `provider/boot.go`:

```go
// boot.go — before
func Boot(app *hex.App) {
    app.Register(
        &Database{},
    )
    // hex:providers
}

// boot.go — after
func Boot(app *hex.App) {
    app.Register(
        &Database{},
    )

    app.Register(
        &Cache{},     // ← added by hex make:provider
    )
    // hex:providers
}
```

The generator finds the `// hex:providers` marker comment and inserts above it — same grouped `Register()` pattern used in CLI's `app/bootstrap.go` and bot's `bot/bootstrap.go` today.

### `hex make:domain` — Generate a domain package

```bash
hex make:domain token
```

Generates a complete domain package following the hexagonal pattern:

```
domain/
└── token/
    ├── token.go          # Entity/aggregate root struct + New() constructor
    ├── repository.go     # Repository interface (Store, Get, List, Delete)
    ├── service.go        # Service struct depending on Repository interface
    └── errors.go         # Sentinel errors (ErrNotFound, etc.)
```

Each file follows hex conventions — domain depends on nothing outside itself.

### `hex make:adapter` — Generate an infrastructure adapter

```bash
hex make:adapter token --repo token.Repository --dialect sqlite
```

Generates `infrastructure/sqlite/token_repository.go`:

```go
package sqlite

import (
    "context"
    "database/sql"

    "myapp/domain/token"
    "github.com/doug-martin/goqu/v9"
)

type TokenRepository struct {
    db *sql.DB
    qb goqu.DialectWrapper
}

func NewTokenRepository(db *sql.DB) *TokenRepository {
    return &TokenRepository{db: db, qb: goqu.Dialect("sqlite3")}
}

func (r *TokenRepository) Store(ctx context.Context, t *token.Token) error {
    panic("not implemented")
}

// ... stubs for all Repository interface methods
```

### `hex make:migration` — Generate a migration

```bash
hex make:migration create_tokens_table
```

Generates timestamped migration files:

```
db/migrations/
├── 20260616120000_create_tokens_table.up.sql
└── 20260616120000_create_tokens_table.down.sql
```

With sensible stubs:

```sql
-- 20260616120000_create_tokens_table.up.sql
CREATE TABLE IF NOT EXISTS tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 20260616120000_create_tokens_table.down.sql
DROP TABLE IF EXISTS tokens;
```

### `hex make:command` — Generate a CLI command

```bash
hex make:command token list
```

Generates `cli/token/list.go`:

```go
package token

import (
    "github.com/jordanbrauer/hex/container"
    "github.com/jordanbrauer/hex/provider"
    "github.com/spf13/cobra"
)

func ListCmd(app provider.Application) *cobra.Command {
    return &cobra.Command{
        Use:   "list",
        Short: "List tokens",
        RunE: func(cmd *cobra.Command, args []string) error {
            // svc := container.Must[*token.Service](app, "token.service")
            // TODO: implement
            return nil
        },
    }
}
```

If `cli/token/` doesn't exist, also generates the parent command group (`cli/token/root.go`) and wires it into the root command.

### `hex make:event` — Generate event types + handler

```bash
hex make:event release.published
```

Generates an event handler file with the subscribe wiring.

### Summary of generators

| Command | What it generates | Auto-wires |
|---------|-------------------|------------|
| `hex init <name>` | Full project skeleton | Everything — ready to `go run` |
| `hex make:provider <name>` | `provider/<name>.go` | Adds to `provider/boot.go` |
| `hex make:domain <name>` | `domain/<name>/` (entity, repo, service, errors) | Nothing — pure domain |
| `hex make:adapter <name>` | `infrastructure/<dialect>/<name>_repository.go` | Nothing — wire in provider |
| `hex make:migration <name>` | `db/migrations/<ts>_<name>.{up,down}.sql` | Nothing — auto-discovered by embed.FS |
| `hex make:command <group> <name>` | `cli/<group>/<name>.go` | Adds to parent command group |
| `hex make:event <name>` | Event handler file | Adds subscriber registration |

### Template engine

Generators use Go's `text/template` with `embed.FS` templates stored inside the hex binary. Template variables come from:

- **Project config** — module path, binary name, database dialect (read from `go.mod` + hex config)
- **Generator args** — entity name, interface to implement, etc.
- **Conventions** — PascalCase for types, snake_case for files, lowercase for packages

No external template dependencies. Templates are embedded at compile time.

### Project detection

All `hex make:*` commands detect the project root by walking up from cwd looking for `go.mod` with a hex dependency. This means you can run generators from any subdirectory.

The hex CLI reads the module path from `go.mod` to generate correct import paths in scaffolded code.

---

## Package Designs

### `hex` (root) — App Kernel

The central orchestrator. Owns the container, provider registry, and event bus. Replaces `app/app.go` (CLI) and `bot/bot.go` (bot).

```go
package hex

type App struct { /* container, registry, bus, bootedAt */ }

func New(opts ...Option) *App

// Lifecycle
func (a *App) Bootstrap(ctx context.Context) error  // Register → Boot all providers
func (a *App) Shutdown(ctx context.Context) error    // Reverse-order shutdown

// Access
func (a *App) Container() *container.Container
func (a *App) Events() *events.Bus
func (a *App) BootedAt() time.Time

// Provider management
func (a *App) Register(providers ...provider.Service)

// Convenience: delegate to container (so app itself satisfies provider.Application)
func (a *App) Bind(name string, fn container.Factory)
func (a *App) Singleton(name string, fn container.Factory)
func (a *App) Make(name string) (any, error)

// Convenience: delegate to events
func (a *App) On(event string, fn events.Subscriber)
func (a *App) Emit(event string, data ...any) error
```

**Options:**

```go
func WithLogger(l *log.Logger) Option       // custom logger instance
func WithEventBus(b *events.Bus) Option     // pre-configured bus
func WithContainer(c *container.Container) Option
```

**Key difference from current code:** Both repos embed `*ioc.Container` directly into their app struct. hex wraps it behind methods instead, so the container API is explicit and the app struct can evolve without breaking embedder assumptions.

**Key difference from current code:** The bot's `Runtime` embeds `*events.Bus` — hex exposes it via `Events()` method and `On()`/`Emit()` convenience methods. Same capability, cleaner boundary.

### `hex/container` — IoC Container

Type-safe dependency injection. Unifies `lib/ioc/container.go` from both repos.

```go
package container

type Factory func(*Container) (any, error)

type Container struct { /* bindings, singletons, mu */ }

func New() *Container

// Registration
func (c *Container) Bind(name string, fn Factory)        // new instance per resolution
func (c *Container) Singleton(name string, fn Factory)   // cached after first resolution

// Resolution
func Make[T any](c *Container, name string) (T, error)   // type-safe
func Must[T any](c *Container, name string) T            // panics on error

// Introspection
func (c *Container) Has(name string) bool
func (c *Container) Count() int
func (c *Container) List() []string
```

**Differences from current implementations:**

| Aspect | CLI | Bot | hex |
|--------|-----|-----|-----|
| Singleton strategy | `sync.Once` per entry | Double-checked lock on map | `sync.Once` per entry (CLI's approach is cleaner) |
| Thread safety | Mutex around all ops | Mutex with manual lock/unlock dance | `sync.RWMutex`, consistent locking |
| Cycle detection | ❌ | ❌ | ✅ Track resolution stack, error on cycle |
| `log.Fatal` in `Must` | Yes (charm log) | Yes (charm log) | `panic()` instead — let consumer catch |

**Design note on `Must` panic vs `log.Fatal`:** Both repos currently call `log.Fatal` inside `Must`, which calls `os.Exit(1)`. This is fine in their main apps but makes the container untestable in isolation (can't catch `os.Exit`). hex uses `panic` so tests can `recover`, and consumers can wrap if they want `Fatal` behavior.

### `hex/provider` — Service Provider Lifecycle

Defines the provider contract and ordered registry. Unifies `lib/provider/*.go` from both repos.

```go
package provider

// Application is the interface that providers interact with during lifecycle hooks.
// Consumer apps typically pass their *hex.App which satisfies this.
type Application interface {
    Bind(string, container.Factory)
    Singleton(string, container.Factory)
    Make(string) (any, error)

    // Event bus access (bot needs this, CLI can ignore it)
    On(string, events.Subscriber)
    Emit(string, ...any) error
}

// Service defines the lifecycle hooks for a service provider.
type Service interface {
    Register(Application) error
    Boot(context.Context, Application) error
}

// Shutdowner is optional — implement only if cleanup is needed.
type Shutdowner interface {
    Shutdown(context.Context, Application) error
}

// Base is a no-op implementation. Embed in concrete providers.
type Base struct{}

func (Base) Register(Application) error                   { return nil }
func (Base) Boot(context.Context, Application) error      { return nil }

// Registry manages ordered provider lifecycle.
type Registry struct { /* providers, booted */ }

func NewRegistry() *Registry
func (r *Registry) Add(providers ...Service)
func (r *Registry) Register(app Application) error       // calls Register on all
func (r *Registry) Boot(ctx context.Context, app Application) error  // calls Boot on all, tracks booted
func (r *Registry) Shutdown(ctx context.Context, app Application)    // reverse-order, only Shutdowner impls
```

**Differences from current implementations:**

| Aspect | CLI | Bot | hex |
|--------|-----|-----|-----|
| `Boot` signature | `Boot(Application) error` | `Boot(Application) error` | `Boot(context.Context, Application) error` — context flows through |
| Shutdown | All providers implement `Shutdown` (no-op default) | Same | Optional `Shutdowner` interface — only call if implemented |
| Event bus in Application | ❌ | `Subscribe` + `Publish` | `On` + `Emit` (renamed for consistency) |
| Base struct name | `Provider` | `Provider` | `Base` — avoids stutter (`provider.Provider` → `provider.Base`) |

### `hex/events` — Typed Event Bus

Currently only the bot has this. hex makes it available to all apps.

```go
package events

type Subscriber func(...any) error

type Bus struct { /* subscribers map, mutex */ }

func New() *Bus

// Subscribe registers a handler. Returns an unsubscribe function.
func (b *Bus) On(event string, fn Subscriber) func()

// Emit publishes to all subscribers. Returns first error encountered.
func (b *Bus) Emit(event string, data ...any) error

// EmitAsync publishes without blocking. Errors are logged.
func (b *Bus) EmitAsync(event string, data ...any)

// Size returns total subscriber count across all events.
func (b *Bus) Size() int
```

**Differences from bot's current `lib/events/bus.go`:**

| Aspect | Bot | hex |
|--------|-----|-----|
| Subscribe return | void | `func()` — unsubscribe function |
| Async emit | ❌ | `EmitAsync` for fire-and-forget |
| Method names | `Subscribe` / `Publish` | `On` / `Emit` (shorter, matches JS conventions the team is familiar with) |
| Snapshot on emit | ✅ `slices.Clone` | ✅ same |

**Future consideration:** Typed generic events (`On[T]` / `Emit[T]`) would eliminate the `...any` casting. Holding off for PoC — the untyped bus is proven in the bot.

### `hex/db` — Database Helpers

Unifies SQLite (CLI) and Postgres (bot) connection + migration patterns.

```go
package db

type Dialect string

const (
    SQLite   Dialect = "sqlite"
    Postgres Dialect = "postgres"
)

type Config struct {
    Dialect    Dialect
    DSN        string
    Migrations embed.FS       // embedded SQL files
    MigrationsDir string     // subdirectory within embed.FS (e.g. "migrations")
    Pragmas    []string       // SQLite-only (e.g. "journal_mode = WAL")
}

// Open connects to the database and runs pending migrations.
func Open(ctx context.Context, cfg Config) (*sql.DB, error)

// OpenMemory opens an in-memory SQLite database with migrations.
func OpenMemory(cfg Config) (*sql.DB, error)

// Migrate runs pending migrations on an existing connection.
func Migrate(db *sql.DB, cfg Config) error

// MigrateDown rolls back all migrations.
func MigrateDown(db *sql.DB, cfg Config) error
```

**Differences from current implementations:**

| Aspect | CLI | Bot | hex |
|--------|-----|-----|-----|
| Dialect | SQLite hardcoded | Postgres hardcoded | Config-driven |
| Migrations source | `embed.FS` | Filesystem path (`file://`) | `embed.FS` always (portable, no deploy path issues) |
| Connection | Returns `(*sql.DB, error)` | Package-level singleton `Connect()` | Returns `(*sql.DB, error)` (CLI's approach — no global state) |
| Pragmas | Hardcoded in `pragmas()` | N/A | Configurable via `Config.Pragmas` |
| In-memory | `OpenMemory()` ✅ | ❌ | `OpenMemory()` ✅ |

### `hex/config` — Configuration

Unifies config loading. Both repos use Viper but wire it differently.

```go
package config

type Config struct {
    // TOML file sources (checked in order, merged; highest priority last)
    Files []string              // e.g. ["/etc/app/config.toml", "~/.config/app/config.toml"]

    // Embedded TOML defaults (loaded first, lowest priority)
    Defaults embed.FS
    DefaultsDir string          // subdirectory within embed.FS

    // Env var mapping (YAML file: config key → env var name)
    // e.g. database.dsn: FINCH_DATABASE_DSN
    EnvMap embed.FS             // embedded env.yaml
    EnvMapFile string           // path within embed.FS (e.g. "env.yaml")

    // Env file (optional, loaded via godotenv for local dev)
    EnvFile string              // e.g. ".env"
}

type Store struct { /* viper instance */ }

func Load(cfg Config) (*Store, error)

// Accessors
func (s *Store) String(key string) string
func (s *Store) Int(key string) int
func (s *Store) Bool(key string) bool
func (s *Store) Duration(key string) time.Duration
func (s *Store) Unmarshal(key string, target any) error
func (s *Store) Viper() *viper.Viper                    // escape hatch
```

**Priority order (highest wins):** env vars (mapped via env.yaml) → user config files (TOML) → embedded defaults (TOML).

**Convention:** App config is always TOML. Env var mapping is always a YAML file (`config/env.yaml`) that declaratively maps config keys to environment variable names. This is the pattern from finch-bot — the YAML file is not application config, it's a binding declaration.

### `hex/build` — Build Info

Standardizes the ldflags pattern both repos use.

```go
package build

// Set via ldflags at compile time:
//   -X hex/build.version=v1.0.0
//   -X hex/build.commit=abc1234
//   -X hex/build.date=2025-01-01T00:00:00Z
//   -X hex/build.branch=main

func Version() string
func Commit() string           // full SHA
func ShortCommit() string      // first 7 chars
func Branch() string
func Time() time.Time
func GoVersion() string
func OS() string
func Arch() string
func Compiler() string
func Debug() bool              // true if `go run` or unset ldflags

// Info returns a formatted multi-line build summary.
func Info() string
```

**Design note:** Unlike the bot's current `init()` which shells out to `git rev-parse` at startup, hex only uses ldflags values. If unset, fields return sensible defaults ("dev", "HEAD", etc.). No `exec.Command` at init time — it's surprising behavior for a library.

### `hex/log` — Logging

Thin wrapper around charmbracelet/log with styled defaults.

```go
package log

// Init configures the global logger with hex's styled defaults.
// Call once at startup, before Bootstrap.
func Init(opts ...Option)

// Options
func WithLevel(level Level) Option
func WithCaller(enabled bool) Option
func WithTimestamp(enabled bool) Option

// Level management
func SetLevel(level Level)
func ParseLevel(s string) (Level, error)

// Re-exported convenience (delegates to charmbracelet/log)
func Debug(msg string, args ...any)
func Info(msg string, args ...any)
func Warn(msg string, args ...any)
func Error(msg string, args ...any)
func Fatal(msg string, args ...any)

// Re-exported level constants
const (
    DebugLevel Level = ...
    InfoLevel  Level = ...
    WarnLevel  Level = ...
    ErrorLevel Level = ...
    FatalLevel Level = ...
)
```

**Design note:** The CLI's `log/log.go` uses `init()` to configure styles. hex uses explicit `Init()` so consumers control when setup happens. The styled ANSI colors for each level match what the CLI already uses.

### `hex/cli` — Cobra Scaffolding

Common patterns for building Cobra CLI apps on top of hex.

```go
package cli

// Root creates a Cobra root command pre-wired with common flags and
// PersistentPreRun that applies --log-level and --env.
func Root(name, short string, app *hex.App) *cobra.Command

// Version returns a standard `version` subcommand that prints build info.
func Version() *cobra.Command

// Flags
func AddLogLevelFlag(cmd *cobra.Command)     // --log-level
func AddVerboseFlag(cmd *cobra.Command)      // --verbose / -v
func AddEnvFlag(cmd *cobra.Command)          // --env / -e
```

## Canonical Project Structure

Every hex project — whether created by `hex init` or migrated manually — follows this layout:

```
myapp/
├── cmd/
│   └── myapp/
│       └── main.go               # Entry point — hex.New(), bootstrap, cli.Root()
├── provider/
│   ├── boot.go                   # Bootstrap: registers all providers in order
│   ├── database.go               # Database provider
│   ├── session.go                # Session provider
│   └── token.go                  # Domain-specific provider
├── domain/
│   ├── token/
│   │   ├── token.go              # Entity/aggregate root
│   │   ├── repository.go         # Port interface
│   │   ├── service.go            # Business logic
│   │   └── errors.go             # Sentinel errors
│   └── shared/                   # Cross-domain value objects
├── infrastructure/
│   ├── sqlite/                   # SQLite adapters
│   │   └── token_repository.go
│   └── api/                      # HTTP adapters
│       └── provider_repository.go
├── cli/
│   ├── root.go                   # Cobra root command
│   └── token/
│       ├── root.go               # Command group
│       └── list.go               # Subcommand
├── db/
│   └── migrations/               # Embedded SQL migrations
│       ├── 001_create_tokens.up.sql
│       └── 001_create_tokens.down.sql
├── config/
│   └── defaults/
│       └── app.toml              # Embedded default config
├── build/
│   └── build.go                  # ldflags variables
├── .env.example
├── go.mod
├── Makefile
└── README.md
```

This is the directory structure `hex init` creates. Generators place files into these directories by convention — you never have to tell them where things go.

## Consumer Migration Example

### Before (finch-cli today)

```
finch-cli/
├── app/
│   ├── app.go              ← kernel (custom)
│   ├── bootstrap.go        ← provider wiring
│   └── provider/*.go       ← service providers
├── lib/
│   ├── ioc/container.go    ← IoC container (duplicated from bot)
│   └── provider/*.go       ← lifecycle interfaces (duplicated)
├── config/repository.go    ← config loading
├── db/connection.go        ← SQLite setup
├── log/log.go              ← logging
└── main.go
```

### After (finch-cli on hex)

```
finch-cli/
├── cmd/
│   └── finch/
│       └── main.go         ← hex.New() + providers + cli.Root()
├── provider/               ← app-specific providers (import hex/provider)
│   ├── boot.go             ← ordered registration
│   ├── database.go
│   ├── session.go
│   ├── token.go
│   └── ...
├── domain/                 ← unchanged
├── infrastructure/         ← unchanged
├── cli/                    ← unchanged (commands)
├── db/
│   └── migrations/         ← app's own migrations (embed.FS)
├── config/
│   └── defaults/           ← embedded TOML defaults
├── build/
│   └── build.go
├── Makefile                ← standardized targets from hex convention
└── go.mod
```

**Deleted from CLI:** `app/` (entire directory — kernel + bootstrap replaced by hex + `provider/boot.go`), `lib/ioc/`, `lib/provider/`, `log/log.go`, `config/repository.go`, `db/connection.go` (open/migrate logic).

**Deleted from bot:** `lib/ioc/`, `lib/provider/`, `lib/events/`, `bot/bot.go` (kernel), `bot/bootstrap.go` (replaced by `provider/boot.go`), `build/init.go` (git exec at init), `db/connection.go` (connect/migrate logic).

### After (main.go)

```go
package main

import (
    "context"
    "os"

    "github.com/jordanbrauer/hex"
    hexcli "github.com/jordanbrauer/hex/cli"
    hexlog "github.com/jordanbrauer/hex/log"

    "finch/cli/auth"
    "finch/cli/token"
    "finch/cli/plugin"
    "finch/provider"
)

func main() {
    hexlog.Init()

    app := hex.New()
    provider.Boot(app)

    if err := app.Bootstrap(context.Background()); err != nil {
        hexlog.Fatal("bootstrap failed", "error", err)
    }
    defer app.Shutdown(context.Background())

    root := hexcli.Root("finch", "Finch developer platform CLI", app)
    root.AddCommand(
        auth.New(app),
        token.New(app),
        plugin.New(app),
        hexcli.Version(),
    )

    if err := root.Execute(); err != nil {
        os.Exit(1)
    }
}
```

```go
// provider/boot.go — same shape as app/bootstrap.go (CLI) and bot/bootstrap.go (bot)
package provider

import "github.com/jordanbrauer/hex"

func Boot(app *hex.App) {
    // Core
    app.Register(&Database{}, &Session{})

    // API
    app.Register(&API{})

    // Domain services
    app.Register(&Token{}, &Auth{}, &Organization{}, &Application{})

    // Plugins
    app.Register(&Lua{})
    // hex:providers
}
```

## Dependencies

```
github.com/spf13/cobra          # CLI framework
github.com/spf13/viper          # Config loading
github.com/charmbracelet/log    # Structured logging
github.com/charmbracelet/lipgloss # Log level styling
github.com/golang-migrate/migrate/v4  # Database migrations
```

Database drivers are **not** hex dependencies. Consumer apps blank-import their own:
- `modernc.org/sqlite` (CLI)
- `github.com/lib/pq` (bot)

All hex dependencies are already used by both consumer apps — hex introduces zero new deps.

The hex CLI tool itself (`cmd/hex`) adds no extra deps beyond what the library already pulls in — it uses `text/template` and `embed` from stdlib for code generation.

## hex Repository Structure

```
hex/
├── app.go                      # App kernel
├── provider.go                 # Provider interface
├── container/
│   ├── container.go
│   └── container_test.go
├── provider/
│   ├── registry.go
│   └── registry_test.go
├── events/
│   ├── bus.go
│   └── bus_test.go
├── db/
│   ├── db.go
│   ├── sqlite.go
│   ├── postgres.go
│   └── migrate.go
├── config/
│   ├── config.go
│   ├── loader.go
│   └── config_test.go
├── build/
│   ├── info.go
│   └── ldflags.go
├── log/
│   ├── log.go
│   └── levels.go
├── cli/
│   ├── root.go
│   ├── flags.go
│   └── version.go
├── cmd/
│   └── hex/                    # ← the scaffolding CLI binary
│       ├── main.go
│       ├── init.go             # hex init command
│       ├── make.go             # hex make:* parent command
│       ├── make_provider.go    # hex make:provider
│       ├── make_domain.go      # hex make:domain
│       ├── make_migration.go   # hex make:migration
│       ├── make_command.go     # hex make:command
│       ├── make_adapter.go     # hex make:adapter
│       ├── make_event.go       # hex make:event
│       ├── project.go          # Project detection (find go.mod, parse module path)
│       ├── generator.go        # Template engine (load, render, write, wire)
│       └── templates/          # Embedded Go templates
│           ├── init/           # Full project scaffold
│           │   ├── main.go.tmpl
│           │   ├── boot.go.tmpl
│           │   ├── root.go.tmpl
│           │   ├── database_provider.go.tmpl
│           │   ├── build.go.tmpl
│           │   ├── Makefile.tmpl
│           │   ├── gitignore.tmpl
│           │   ├── env.example.tmpl
│           │   ├── config.toml.tmpl
│           │   └── README.md.tmpl
│           ├── provider.go.tmpl
│           ├── domain/
│           │   ├── entity.go.tmpl
│           │   ├── repository.go.tmpl
│           │   ├── service.go.tmpl
│           │   └── errors.go.tmpl
│           ├── adapter.go.tmpl
│           ├── migration.up.sql.tmpl
│           ├── migration.down.sql.tmpl
│           ├── command.go.tmpl
│           ├── command_group.go.tmpl
│           └── event_handler.go.tmpl
├── go.mod
├── go.sum
└── README.md
```

## Implementation Phases

### Phase 1 — Core (container + provider + app kernel)

Everything depends on this. Port the IoC container, define the provider interface, build the app kernel.

**Packages:** `hex/container`, `hex/provider`, `hex` (root)
**Tests:** Container resolution, singleton caching, cycle detection, provider lifecycle ordering, reverse shutdown.

### Phase 2 — Events + Log + Build

Quick wins with small surface area. The event bus is self-contained. Logging and build info are leaf packages with no internal deps.

**Packages:** `hex/events`, `hex/log`, `hex/build`
**Tests:** Event subscribe/emit/unsubscribe, async emit, log level parsing, build info formatting.

### Phase 3 — Config + DB

Config and database helpers. These depend on the container (providers bind them) but are otherwise standalone.

**Packages:** `hex/config`, `hex/db`
**Tests:** Config priority (env > file > defaults), SQLite open/migrate, Postgres open/migrate, in-memory SQLite.

### Phase 4 — CLI scaffolding library

Cobra helpers for consumer apps. Depends on the app kernel and config (for `--log-level` flag).

**Packages:** `hex/cli`
**Tests:** Root command creation, version output, flag parsing.

### Phase 5 — Cron

A scheduler with cron expression parsing and named jobs. Provider-friendly (Start/Stop lifecycle maps to Boot/Shutdown).

**Package:** `hex/cron`
**Tests:** Job registration, cron expression parsing, tick behaviour with a virtual clock.

### Phase 6 — Cache

Multi-backend key-value cache. Interface + `memory` backend in v1; `redis`/`valkey`/`memcached` as opt-in subpackages under `hex/cache/*` following the `hex/db/sqlite`+`hex/db/postgres` pattern.

**Package:** `hex/cache`, `hex/cache/memory`, `hex/cache/redis`
**Tests:** Get/Set/Delete/TTL semantics, atomic increments, eviction.

### Phase 7 — Disk

Laravel-style multi-backend filesystem (ADR-0008). Interface + `local` backend in v1; S3/minio/GCS as opt-in subpackages once local is stable.

**Package:** `hex/disk`, `hex/disk/local` (later: `hex/disk/s3`)
**Tests:** Read/Write/Exists/Delete/List/URL against a temp dir; interface coverage against multiple backends when they exist.

### Phase 8 — TUI

Styles + markup + renderer + console helpers ported from finch-cli's `tui/` tree.

**Package:** `hex/tui`, and subpackages as needed (`tui/markup`, `tui/renderer`, `tui/styles`).
**Tests:** Golden-file rendering, style token resolution.

### Phase 9 — Web

Echo-backed HTTP server with the standard middleware stack (ADR-0006): request ID, structured logging, panic recovery, CORS, plus `/healthz` and `/readyz`. Graceful shutdown wired to `app.Shutdown` via a `Shutdowner` provider.

**Package:** `hex/web`
**Tests:** Middleware behaviour, health endpoint responses, shutdown ordering.

### Phase 10 — Lua

Runtime-only (ADR-0007): compile, load, execute scripts. No bindings, no plugin system.

**Package:** `hex/lua`
**Tests:** Script compilation cache, error propagation with Lua stack traces, panic isolation across scripts.

### Phase 11 — Queue

Layered per ADR-0009: generic `Queue` interface (Publish/Subscribe over topic+[]byte) with a `Jobs` layer on top for named jobs with retry/backoff/DLQ/delayed dispatch. Backends in v1: `memory` (in-process, for tests) and `sqlite` (durable, reuses hex/db/sqlite). Postgres, SQS/SNS, RabbitMQ, Kafka land later as opt-in subpackages.

**Packages:** `hex/queue`, `hex/queue/jobs`, `hex/queue/memory`, `hex/queue/sqlite`
**Tests:** Publish/Subscribe roundtrip, at-least-once delivery semantics, job retry with backoff, dead-letter routing, delayed dispatch, concurrent consumers.

### Phase 12 — Pool

Worker pool primitive wrapping alitto/pond. Provides bounded in-process concurrency for fan-out patterns, HTTP handlers, and (eventually) queue consumers.

**Package:** `hex/pool`
**Tests:** Submit / SubmitErr semantics, groups with context, panic recovery, StopAndWait draining, metrics accuracy.

### Phase 13 — Policy

Authorisation wrapper around Casbin (ADR-0011). Ships memory and file adapters in v1; SQL adapter deferred to the same pattern as hex/db subpackages.

**Package:** `hex/policy`
**Tests:** RBAC + ABAC model enforcement, adapter round-trips, policy add/remove at runtime, model reload.

### Phase 14 — i18n

GNU gettext-compatible i18n via gotext (ADR-0012). Multi-locale Translator with fallback + package-level `T`/`TN`/`TC` backed by SetDefault. Locales load from disk or `fs.FS`.

**Package:** `hex/i18n`
**Tests:** PO round-trip, plurals, msgctxt, missing translations fall back to msgid, multi-locale switching, embed.FS load.

### Phase 15 — Feature flags

Feature-flagging via GOFF (ADR-0013). Ships file + embed.FS retrievers in v1; consumers pull other retrievers direct from GOFF and pass through Options.

**Package:** `hex/featureflag`
**Tests:** Bool/Int/String/Float64/JSON variation with default fallback, rule-based targeting, embed.FS retriever, missing flag returns default.

### Phase 16 — Essentials batch

Eight small, mostly-independent packages that batteries-included frameworks ship. Batched because each is under ~200 lines and they share test/doc patterns.

**Packages:** `hex/clock`, `hex/id`, `hex/errors`, `hex/hash`, `hex/retry`, `hex/ratelimit`, `hex/httpx`, `hex/validate`

### Phase 17 — Telemetry

OpenTelemetry setup wrapper. Provides configured tracer + meter + a logger bridge from hex/log so spans carry log correlation.

**Package:** `hex/telemetry`

### Phase 18 — BDD test support

BDD runner wrapping github.com/go-bdd/gobdd. Type aliases + hex-owned constructors + embed.FS support so consumers can ship `.feature` files inside binaries.

**Package:** `hex/bdd`

### Phase 19 — hex CLI tool (`hex init` + generators)

The scaffolding CLI itself. This is the user-facing `hex` binary that generates projects and code.

**Package:** `cmd/hex` (compiles to `hex` binary)
**Templates:** Embedded via `embed.FS` in `cmd/hex/templates/`
**Commands:**

| Command | Priority | Complexity |
|---------|----------|------------|
| `hex init` | P0 — must ship first | Medium (full project scaffold) |
| `hex make:provider` | P0 | Low (single file + boot.go wire) |
| `hex make:domain` | P0 | Low (4 files, no wiring) |
| `hex make:migration` | P0 | Trivial (timestamped SQL stubs) |
| `hex make:command` | P1 | Medium (parent group detection) |
| `hex make:adapter` | P1 | Medium (reads interface, generates stubs) |
| `hex make:event` | P2 | Medium (subscriber wiring) |

**Tests:** Golden file tests — run each generator, compare output against checked-in snapshots. `UPDATE_SNAPSHOTS=true go test ./...` to refresh.

### Phase 20 — Migrate finch-cli

First real consumer. Replace `app/`, `lib/ioc`, `lib/provider`, `config/repository.go`, `db/connection.go`, `log/log.go` with hex imports and the canonical project structure. This validates both the library API and the generated structure against a real, complex app.

### Phase 21 — Migrate finch-bot

Second consumer. Replace `lib/ioc`, `lib/provider`, `lib/events`, `bot/bot.go`, `bot/bootstrap.go`, `build/*.go`, `db/connection.go` with hex imports. Validates that the same framework serves both a CLI tool and a long-running service.

## Open Questions

1. ~~**Module path**~~ — Decided: `github.com/jordanbrauer/hex`. Personal project, not org-scoped.

2. ~~**Go version**~~ — Decided: Go 1.25 (latest). Bot bumps to 1.25 when it adopts hex.

3. ~~**Goqu**~~ — Decided: Consumer-side. hex/db doesn't depend on goqu.

4. ~~**Viper vs koanf**~~ — Decided: Viper. Both repos already use it. Wrapped behind `hex/config.Store` so the implementation can swap later without consumer changes.

5. ~~**License**~~ — Decided: MIT.

6. ~~**Generator auto-wiring strategy**~~ — Decided: Marker comments (e.g. `// hex:providers`). Generator finds the marker, inserts above it. If marker is missing, generator creates the file and tells the user to wire manually. AST parsing rejected because `go/printer` reformats the file and destroys comment groupings.

7. ~~**Template customization**~~ — Decided: No. hex is opinionated — one set of templates, embedded in the binary. Custom templates can be added later without breaking anything.

8. ~~**`hex new` vs `hex init`**~~ — Decided: `hex init` only. `hex init myapp` creates dir + scaffolds, `hex init` or `hex init .` scaffolds in cwd. One command, same as `go mod init` pattern.
