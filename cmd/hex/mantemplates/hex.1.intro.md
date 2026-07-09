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
