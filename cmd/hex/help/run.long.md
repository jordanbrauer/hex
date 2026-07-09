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
