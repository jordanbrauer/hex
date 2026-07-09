---
title: hex
section: 5
header: hex manual
footer: hex
---

# NAME

hex - configuration file formats for hex applications

# DESCRIPTION

A hex application is configured from its `config/` directory. Configuration is
layered from three sources, and each concern uses exactly one file format: TOML
for values, YAML for the env map, and CUE for the schema. This split, and the
layering, are fixed by the framework (see ADR-0005).

# LAYERING

Values are resolved from three layers. Later layers override earlier ones:

1. **Framework defaults** — each provider ships its own defaults (embedded
   TOML), lowest priority.
2. **Application config** — the project's own `config/*.toml` files.
3. **Environment variables** — bound through the env map, highest priority.

Publish a provider's defaults into `config/` to inspect or override them with
`hex publish <component>` (see **hex**(1)).

# TOML CONFIG (config/*.toml)

Each `.toml` file is a **namespace**, named after the file. A key is addressed
as `<namespace>.<key>`. For example `config/database.toml`:

```
[database]
dsn = "file:app.db?_pragma=journal_mode(WAL)"
```

is read as `database.dsn`. Framework namespaces include `database`, `server`,
`log`, `cache`, and `telemetry`; the application adds its own (for example
`app`).

# ENV MAP (config/env.yaml)

The env map is a **binding declaration**, not application config. It maps dotted
config keys to the environment variable that overrides them:

```
database.dsn: MYAPP_DATABASE_DSN
log.level: MYAPP_LOG_LEVEL
```

At runtime, if `MYAPP_DATABASE_DSN` is set, its value wins over the TOML file
and the framework default. Only keys listed here are overridable from the
environment; the framework itself reads no environment variables directly.

# SCHEMA (config/schema.cue)

`config/schema.cue` validates configuration with CUE. The application declares
constraints for its **own** namespaces here; framework namespaces already ship
their own schemas alongside each provider's defaults, so they need not be
repeated. Configuration that violates the schema fails fast at load time.

# FILES

`config/app.toml`
:   the application's own namespace.

`config/*.toml`
:   one namespace per file (`database.toml`, `server.toml`, `log.toml`, …).

`config/env.yaml`
:   the env-var binding map.

`config/schema.cue`
:   CUE validation for app-owned namespaces.

`config/config.go`
:   embeds the above via `//go:embed` for a single-binary deploy.

# SEE ALSO

**hex**(1) for the CLI, **hex**(7) for the framework conventions, and
**hex**(3) for the embedded Lua API.

hex configuration is fixed by ADR-0005 in the framework repository.
