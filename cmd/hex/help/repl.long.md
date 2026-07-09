Launch an interactive REPL that evaluates Teal by default. Use `--mode` to start
in Lua or Fennel instead.

In interactive mode, switch languages on the fly at an empty prompt: `t` for
Teal, `l` for Lua, `f` for Fennel. Backspace on an empty prompt in a non-default
mode returns to the language you launched with.

The runtime is bare gopher-lua plus the requested compiler; no hex modules are
pre-loaded. Scaffolded apps get a container-aware REPL via `<appname> repl`,
which has access to `db`, `cache`, `config`, and whatever the app registers.

Exit with Ctrl+D, `exit`, or `quit`.
