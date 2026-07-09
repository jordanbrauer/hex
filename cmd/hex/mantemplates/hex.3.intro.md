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
