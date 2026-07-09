---
title: hex
section: 7
header: hex manual
footer: hex
---

# NAME

hex - conventions for building applications on the hex framework

# DESCRIPTION

hex is an opinionated Go application framework: an IoC container, service
providers with a Register → Boot → Shutdown lifecycle, a typed event bus,
layered config, logging, an HTTP server, a view engine, an embedded Lua
runtime, a queue, a scheduler, and more, behind coherent interfaces. This page
describes the conventions a hex application follows. Generate code with
**hex**(1) rather than hand-authoring it — the generators place files in the
locations described here and wire them in for you.

# PROJECT LAYOUT

A hex project has a fixed shape. Each directory has one responsibility.

`app/`
:   Application wiring. `app/boot.go` registers providers in order;
    `app/provider/` holds the app's service providers; `app/command/` is the
    Cobra command tree; `app/controller/` holds HTTP controllers;
    `app/build/` carries build metadata.

`domain/`
:   Business logic. Each `domain/<name>/` package contains the entity, a
    `Repository` port interface, a `Service`, and sentinel errors. Domain code
    depends on nothing outside itself — no database, no HTTP, no framework.

`infrastructure/`
:   Adapters that implement domain ports (for example a SQL-backed
    `Repository`). This is the only layer that imports drivers.

`config/`
:   TOML configuration, a CUE schema, and the env map. See **hex**(5).

`database/`
:   `migrations.go` plus timestamped `migrations/*.sql` files.

`lib/`
:   Local utility packages that are not domain logic.

# MARKER COMMENTS

Generators auto-wire new code by inserting a line above a marker comment. The
marker must be the first non-whitespace token on its own line. **Never remove
these markers** — they are how **hex**(1) finds where to register new code.

`// hex:providers`
:   in `app/boot.go` — provider registrations.

`// hex:commands`
:   in `app/command/root.go` — top-level command registrations. Command
    groups get their own `// hex:commands:<group>` marker in the group's
    `root.go`.

`// hex:routes`
:   in `app/provider/routes.go` — HTTP route registrations.

`// hex:repl`
:   in `app/provider/repl_bindings.go` — Lua REPL module bindings.

# PROVIDER LIFECYCLE

A service provider is a struct (usually embedding `provider.Base`) with up to
three phases, run in registration order (Shutdown in reverse):

**Register**
:   Bind singletons into the container. Cheap: no I/O, no goroutines. Do not
    call `container.Make` here — a dependency may not be bound yet, and
    singleton errors are cached. Defer resolution into the factory closure.

**Boot**
:   Open resources, run migrations, register routes, start goroutines. All
    expensive work goes here.

**Shutdown**
:   Reverse-order cleanup. Implement only if the provider owns a resource.

Dependencies are resolved from the container by string name — for example
`container.Make[*sql.DB](app, "db")` or `container.Make[*web.Server](app,
"http")`.

# CONVENTIONS

- **No import aliases** unless disambiguating two hex packages; generated code
  never uses them.
- **Test fixtures** live as real files under `testdata/`, embedded via
  `//go:embed` — never inline literals.
- **Vocabulary is fixed.** Use the framework's terms (App, Container, Binding,
  Provider, Registry, Bootstrap, Disk, Cache, Job, Pool, Queue, …); do not
  invent synonyms.

# VERIFICATION

Drive the app end-to-end, not just the unit tests. A scaffolded project ships a
`justfile`:

```
just check      # fmt-check + lint + vet + race tests — the pre-commit gate
go run . serve  # boot the app (with the web component)
go test ./...   # includes webtest end-to-end route tests
```

Route tests use the fluent `webtest` client to boot the real kernel in-process
and assert on HTTP responses and rendered DOM. See the `swapi` example app for
a complete, conventional reference.

# SEE ALSO

**hex**(1) for the CLI, **hex**(5) for config file formats, and
**hex**(3) for the embedded Lua API.
