---
title: hex
section: 1
header: hex manual
footer: hex
---

# NAME

hex - scaffolding CLI for the hex application framework

# SYNOPSIS

**hex** *command* [*arguments*] [*flags*]

# DESCRIPTION

hex is an opinionated Go application framework and a companion CLI that
scaffolds and extends applications built on it. The `init` command creates a
complete, runnable project; the `make:*` generators add correctly-placed,
correctly-wired code for providers, domains, migrations, commands, adapters,
and controllers.

**Prefer the generators over hand-authoring source.** They read the project's
`go.mod` to compute import paths, apply the framework's naming conventions, and
auto-wire new code into the application — the parts that are easy to get wrong
by hand. Every generating command accepts `--dry-run` to preview the files it
would write and wire without touching disk, and `--format json` to emit that
plan as machine-readable output for tooling and agents.

# CONVENTIONS

A hex project has a fixed layout: `app/` (the kernel wiring, providers,
commands, and controllers), `domain/` (pure business logic and port
interfaces), `infrastructure/` (adapters that implement those ports),
`config/` (TOML config, CUE schema, and the env map), and `database/`
(migrations).

Generators auto-wire code by inserting above marker comments —
`// hex:providers` in `app/boot.go`, `// hex:commands` in
`app/command/root.go`, and `// hex:routes` in `app/provider/routes.go`. **Never
remove these markers**; they are how `hex make:*` finds where to register new
code.

See **hex**(7) for the full conventions guide, **hex**(5) for the config
file formats, and **hex**(3) for the embedded Lua API.

# COMMANDS

## hex init

Scaffold a new hex application in the given directory (or the current one).

Run without a name to scaffold into `.`; otherwise a subdirectory named `<name>`
is created. Interactive prompts fill in the Go module path, the binary name, the
database dialect, and which framework components to enable. Pass `--yes` to skip
the prompts and take the flag defaults instead.

The generated project is runnable immediately: it wires an app kernel, a
provider registry, config, logging, and an embedded Lua REPL, and drops the
`// hex:*` marker comments that `hex make:*` uses to auto-wire new code.

Usage:

```
hex init [name] [flags]
```

Options:

`--ai` *string*
:   AI provider: openai, anthropic, none (default "none")

`--cache`
:   scaffold a cache provider (memory backend)

`--cron`
:   scaffold a cron scheduler provider

`--db` *string*
:   database dialect: sqlite, postgres, none (default "sqlite")

`--dry-run`
:   print the actions without writing any files

`--featureflag`
:   scaffold a featureflag (GOFF) provider

`--force`
:   overwrite existing files

`--format` *string*
:   output format: text or json (default "text")

`--frontend` *string*
:   frontend stack: js (no build), ts (Laravel Mix), none (default) (default "none")

`--i18n`
:   scaffold an i18n (gotext) provider

`--module` *string*
:   Go module path (default: github.com/<user>/<name>)

`--policy`
:   scaffold a policy (Casbin) provider

`--queue` *string*
:   queue backend: memory or none (default "none")

`--telemetry` *string*
:   telemetry exporter: stdout, otlp, none (default "none")

`--web`
:   scaffold a web (echo) server provider

`--yes`
:   skip interactive prompts, use flag defaults

Examples:

```sh
# Interactive: prompts for module path, binary name, and components
hex init myapp

# Non-interactive with an embedded SQLite database and web server
hex init myapp --db sqlite --web --yes

# Scaffold into the current directory
hex init . --yes

# A batteries-on service: queue, cron, policy, i18n, flags, telemetry, AI
hex init myapp --web --queue memory --cron --policy --i18n \
  --featureflag --telemetry stdout --ai anthropic --yes
```

## hex make:adapter

Generate an infrastructure adapter at
`infrastructure/<dialect>/<domain>_repository.go` — a stub implementation of
`domain/<domain>.Repository` backed by the given SQL dialect.

The generator produces `panic("not implemented")` stubs for the standard
`Repository` methods (Store, Get, List, Delete) that `hex make:domain` scaffolds,
plus a compile-time `var _ <domain>.Repository = (*…)(nil)` assertion. If you
have extended the interface, add the extra methods by hand.

Usage:

```
hex make:adapter <domain> [flags]
```

Options:

`--dialect` *string*
:   SQL dialect: sqlite or postgres (default "sqlite")

`--dry-run`
:   print the actions without writing any files

`--force`
:   overwrite existing files

`--format` *string*
:   output format: text or json (default "text")

Examples:

```sh
# A SQLite adapter for domain/order.Repository
hex make:adapter order

# A Postgres adapter
hex make:adapter order --dialect postgres
```

## hex make:command

Generate a Cobra command wired into the application.

Without `--group`, the command lands at `app/command/<name>.go` and is registered
against the root's `// hex:commands` marker.

With `--group`, the command lands at `app/command/<group>/<name>.go` and is
registered against the group's `// hex:commands:<group>` marker. The group's
`root.go` is generated automatically the first time and wired into the top-level
command; existing group roots are never overwritten, preserving prior
registrations.

Usage:

```
hex make:command <name> [flags]
```

Options:

`--dry-run`
:   print the actions without writing any files

`--force`
:   overwrite existing files

`--format` *string*
:   output format: text or json (default "text")

`--group` *string*
:   parent command group (creates a subcommand)

Examples:

```sh
# A top-level command: app/command/migrate.go, wired into root
hex make:command migrate

# A grouped subcommand: app/command/user/create.go under a "user" group
hex make:command create --group user

# Preview the files and wiring
hex make:command create --group user --dry-run
```

## hex make:controller

Generate an HTTP controller at `app/controller/<name>.go` and wire its routes
into `app/provider/routes.go` above the `// hex:routes` marker.

By default a single `Index` handler with a `GET /<name>` route is scaffolded.
Use `--all` for full RESTful CRUD (index/show/store/update/destroy) or
`--actions` to pick a comma-separated subset.

Requires `--web` to have been enabled at `hex init` so the Routes provider and
the `app/controller/` package already exist.

Usage:

```
hex make:controller <name> [flags]
```

Options:

`--actions` *string*
:   comma-separated list of actions (index,show,store,update,destroy)

`--all`
:   scaffold full RESTful CRUD (index/show/store/update/destroy)

`--dry-run`
:   print the actions without writing any files

`--force`
:   overwrite existing files

`--format` *string*
:   output format: text or json (default "text")

Examples:

```sh
# A single Index handler + GET /users route
hex make:controller users

# Full RESTful CRUD
hex make:controller users --all

# A chosen subset of actions
hex make:controller users --actions index,show,store

# Preview the controller and route wiring without writing
hex make:controller users --all --dry-run
```

## hex make:domain

Generate a domain package at `domain/<name>/` containing the entity
(`<name>.go`), the `Repository` port interface (`repository.go`), the `Service`
use-cases (`service.go`), and sentinel errors (`errors.go`).

The name is normalised to a lower-case package name and a PascalCase type. The
domain package depends on nothing outside itself — infrastructure implements the
`Repository` port via `hex make:adapter`.

Usage:

```
hex make:domain <name> [flags]
```

Options:

`--dry-run`
:   print the actions without writing any files

`--force`
:   overwrite existing files

`--format` *string*
:   output format: text or json (default "text")

Examples:

```sh
# Generate domain/order/{order,repository,service,errors}.go
hex make:domain order

# Preview the four files without writing them
hex make:domain order --dry-run
```

## hex make:migration

Generate a timestamped migration pair at
`database/migrations/<timestamp>_<name>.{up,down}.sql`.

The timestamp is in the format golang-migrate expects (`yyyyMMddHHmmss`),
lexically sortable and unique to the second. When the name follows the
`create_<table>_table` convention the stubs are pre-filled with a matching
`CREATE TABLE` / `DROP TABLE`; edit them to add your real schema.

Usage:

```
hex make:migration <name> [flags]
```

Options:

`--dry-run`
:   print the actions without writing any files

`--force`
:   overwrite existing files

`--format` *string*
:   output format: text or json (default "text")

Examples:

```sh
# create_users_table -> a CREATE TABLE users / DROP TABLE users pair
hex make:migration create_users_table

# A free-form migration name
hex make:migration add_index_to_orders
```

## hex make:provider

Generate a service provider at `app/provider/<name>.go` and wire it into
`app/boot.go` above the `// hex:providers` marker.

The name is normalised to PascalCase for the type and lower-case snake_case for
the filename. The generated provider embeds `provider.Base`; add your bindings in
`Register` and open resources or start goroutines in `Boot`.

Usage:

```
hex make:provider <name> [flags]
```

Options:

`--dry-run`
:   print the actions without writing any files

`--force`
:   overwrite existing files

`--format` *string*
:   output format: text or json (default "text")

Examples:

```sh
# Generate app/provider/payments.go and wire it into app/boot.go
hex make:provider payments

# Preview what would be written and wired, without touching disk
hex make:provider payments --dry-run

# Machine-readable output for tooling / agents
hex make:provider payments --dry-run --format json
```

## hex publish

Copy the config files that a hex framework provider ships (via its embedded
`Configs()` fs.FS) into your project's `config/` directory so you can inspect and
edit them. Files are copied as-is; the framework's original defaults still apply
as a fallback when your local copy is missing a field.

Pass `--all` to publish every framework component at once. Pass `--force` to
overwrite files you have already published.

Components: cache, db, log, telemetry, web.

Usage:

```
hex publish [component] [flags]
```

Options:

`--all`
:   publish every framework component

`--dry-run`
:   print the actions without writing any files

`--force`
:   overwrite existing files

`--format` *string*
:   output format: text or json (default "text")

Examples:

```sh
# Publish the web server's default config into config/
hex publish web

# Publish every framework provider's configs
hex publish --all

# Overwrite configs you have already published
hex publish web --force
```

## hex repl

Launch an interactive REPL that evaluates Teal by default. Use `--mode` to start
in Lua or Fennel instead.

In interactive mode, switch languages on the fly at an empty prompt: `t` for
Teal, `l` for Lua, `f` for Fennel. Backspace on an empty prompt in a non-default
mode returns to the language you launched with.

The runtime is bare gopher-lua plus the requested compiler; no hex modules are
pre-loaded. Scaffolded apps get a container-aware REPL via `<appname> repl`,
which has access to `db`, `cache`, `config`, and whatever the app registers.

Exit with Ctrl+D, `exit`, or `quit`.

Usage:

```
hex repl [flags]
```

Options:

`--mode` *string*
:   starting language: teal (default), lua, fennel

Examples:

```sh
# Start the REPL in Teal (the default)
hex repl

# Start in Lua
hex repl --mode lua

# Start in Fennel
hex repl --mode fnl
```

## hex run

Run an arbitrary Lua (`.lua`), Teal (`.tl`), or Fennel (`.fnl`) script or inline
code through hex's embedded runtime.

Source can come from three mutually-exclusive places: a file argument (the
extension picks the language), `-` to read from stdin, or `-c` for inline code.
`--lang` forces the language for inline/stdin source (it is ignored for file
arguments, where the extension wins).

Use `--check` to validate without executing: the Teal type-checker for `.tl`,
the Fennel compiler for `.fnl`, and the Lua parser for `.lua`.

The runtime is bare gopher-lua plus the requested compiler; no hex modules
(`db`, `cache`, `agent`, …) are pre-registered. For app-scoped execution with
access to registered modules, use the `repl` command on your scaffolded app.

Usage:

```
hex run [file] [flags]
```

Options:

`--check`
:   validate syntax/types without executing

`--code`, `-c` *string*
:   inline source code (mutually exclusive with a file arg)

`--lang` *string*
:   force language for inline/stdin source: lua, teal, fennel (irrelevant for file args)

Examples:

```sh
# Run a Teal script (language inferred from the extension)
hex run script.tl

# Run inline Lua
hex run -c 'print("hello from lua")'

# Run inline Fennel
hex run -c '(print "hello")' --lang fnl

# Pipe a script in via stdin
echo 'print(1 + 1)' | hex run -

# Type-check a Teal file without running it
hex run script.tl --check
```

# SEE ALSO

**hex**(3) for the embedded Lua API, **hex**(5) for the config
file formats, and **hex**(7) for the framework conventions guide.
