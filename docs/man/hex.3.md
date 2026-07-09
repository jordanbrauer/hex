---
title: hex
section: 3
header: hex manual
footer: hex
---

# NAME

hex - embedded Lua API for hex applications

# DESCRIPTION

hex embeds a Lua runtime (Lua, Teal, and Fennel) that applications built on the
framework expose through scripts and an interactive REPL. This page documents
the Lua modules the framework installs, each loadable with `require`.

The base **hex run** and **hex repl** commands use a bare runtime with no
modules pre-registered. The modules below are installed by their respective
providers and are available in a scaffolded application's own REPL
(`<appname> repl`) and scripts, once the corresponding provider is registered.

Function signatures are given verbatim in Teal notation: `function(arg: type):
return`. A trailing `string` return is, by hex convention, an error message that
is empty on success.

# MODULES

## cache

Type stub for the `cache` Lua module installed by hex/cache/provider.

`cache.get`
:   `function(key: string): (string, boolean, string)`

`cache.set`
:   `function(key: string, value: string, ttl_seconds: number): (boolean, string)`

`cache.delete`
:   `function(key: string): (boolean, string)`

`cache.has`
:   `function(key: string): (boolean, string)`

`cache.clear`
:   `function(): (boolean, string)`

`cache.increment`
:   `function(key: string, delta: integer): (integer, string)`

## config

Type stub for the `config` Lua module installed by hex/lua/provider.

`config.string`
:   `function(key: string): string`

`config.int`
:   `function(key: string): integer`

`config.bool`
:   `function(key: string): boolean`

`config.float`
:   `function(key: string): number`

`config.duration`
:   `function(key: string): number`

`config.stringSlice`
:   `function(key: string): {string}`

`config.set`
:   `function(key: string, value: any): (boolean, string)`

`config.has`
:   `function(key: string): boolean`

`config.namespaces`
:   `function(): {string}`

## db

Type stub for the `db` Lua module installed by hex/db/provider. Read at REPL / hex-run startup so Teal typechecks require("db").

`db.query`
:   `function(sql: string, ...: any): ({{string:any}}, string)`

`db.queryOne`
:   `function(sql: string, ...: any): ({string:any}, string)`

`db.exec`
:   `function(sql: string, ...: any): (Result, string)`

`db.transaction`
:   `function(function(Executor)): (boolean, string)`

## env

Type stub for the `env` Lua module installed by hex/lua/provider.

`env.name`
:   `string`

`env.current`
:   `function(): string`

`env.is_development`
:   `function(): boolean`

`env.is_test`
:   `function(): boolean`

`env.is_production`
:   `function(): boolean`

## events

Type stub for the `events` Lua module installed by hex/lua/provider.

`events.emit`
:   `function(name: string, payload: any): (boolean, string)`

## log

Type stub for the `log` Lua module installed by hex/lua/provider.

`log.debug`
:   `function(msg: string, attrs: {string:any})`

`log.info`
:   `function(msg: string, attrs: {string:any})`

`log.warn`
:   `function(msg: string, attrs: {string:any})`

`log.error`
:   `function(msg: string, attrs: {string:any})`

## queue

Type stub for the `queue` Lua module installed by hex/queue/provider.

`queue.publish`
:   `function(topic: string, body: string): (string, string)`

# TYPES

## Result

A table used by the `db` module.

`rows_affected`
:   `integer`

`last_insert_id`
:   `integer`

## Executor

A table used by the `db` module.

`query`
:   `function(sql: string, ...: any): ({{string:any}}, string)`

`queryOne`
:   `function(sql: string, ...: any): ({string:any}, string)`

`exec`
:   `function(sql: string, ...: any): (Result, string)`

# ENVIRONMENT

*hex reads no environment variables of its own.* Application configuration is supplied through config files and, in scaffolded apps, a declarative env map — see **hex**(5).

# SEE ALSO

**hex**(1), **hex**(5), **hex**(7)
