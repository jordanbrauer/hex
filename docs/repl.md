# REPL

Every scaffolded hex app ships a `<app> repl` command — the
Tinker / Rails console / Phoenix IEx analogue for hex.

```
$ myapp repl
myapp repl — teal mode. Ctrl+D or "exit" to quit.
note: framework modules (db, cache, config, log, env, events, queue)
      are typed globals. ...

myapp(teal)> log.info("hi", { from = "repl" })
INFO hi from=repl
myapp(teal)> local rows = db.query("SELECT id, name FROM users LIMIT 3")
myapp(teal)> for _, u in ipairs(rows) do print(u.id, u.name) end
```

## Modes

- **Teal** (default): strict typechecking against the `.d.tl` stubs
  each framework module ships. Argument type errors are caught
  before execution. Prompt color: `#3e8b9b` (teal-tinted cyan).
- **Lua** (`--mode lua` or `repl.toml`: `mode = "lua"`): looser Lua
  semantics; implicit globals allowed; no typecheck. Use this when
  Teal's strictness gets in the way for a quick prototype.
  Prompt color: `#000080` (Lua's classic navy).

### Switching modes at runtime (Julia-style)

At an **empty prompt**, single-key activators swap modes without
quitting the REPL:

| Key         | Effect                                          |
|-------------|-------------------------------------------------|
| `t`         | switch to Teal mode                             |
| `l`         | switch to Lua mode                              |
| Backspace   | return to the mode you launched with (default)  |

The activator only fires when the input line is empty — typing `l`
mid-word still inserts an `l`. The prompt color updates immediately
so you can see which language your next line will run against.

The Lua environment is shared, so globals set in one mode are
visible from the other (subject to Teal's chunk-local locals
caveat). Framework modules stay registered in both.

## Built-in modules

Wired automatically — no `require()` needed, they're pre-declared as
typed globals in Teal mode.

| Global    | Provides                                                        |
|-----------|-----------------------------------------------------------------|
| `config`  | `config.string(k)`, `config.int(k)`, `config.set(k, v)`, ...    |
| `log`     | `log.info(msg, attrs)`, `log.debug`, `log.warn`, `log.error`    |
| `env`     | `env.current()`, `env.is_production()`, ...                     |
| `events`  | `events.emit(name, payload)`                                    |
| `db`      | `db.query`, `db.queryOne`, `db.exec`, `db.transaction`          |
| `cache`   | `cache.get`, `cache.set(k, v, ttl?)`, `cache.increment`, ...    |
| `queue`   | `queue.publish(topic, body)`                                    |

Only `db`, `cache`, and `queue` require the corresponding scaffold
flag (`--db`, `--cache`, `--queue`) to be present. The others always
ship.

## Persistent state across REPL lines

Teal uses standard Lua chunk semantics: **locals die at the end of
their chunk**. Each REPL line is its own chunk.

```lua
myapp(teal)> local rows = db.query("...")     -- ok
myapp(teal)> print(#rows)                     -- ERROR: rows unknown
```

Two ways to persist:

```lua
-- 1. Use globals (Teal requires explicit declaration):
myapp(teal)> global rows = db.query("...")
myapp(teal)> print(#rows)                     -- ok

-- 2. Chain into one line:
myapp(teal)> print(#db.query("..."))          -- ok
```

Framework modules are already pre-declared as globals, so
`db.query`, `cache.get`, etc. work across lines without any setup.

## Adding your own domain modules

`app/provider/repl_bindings.go` is where you expose your services to
the REPL. Every scaffold ships with this file + a commented example.

The pattern per module:

```go
env.SetType("users", `
    local record User
        id: integer
        email: string
    end
    local record users
        count: function(): (integer, string)
        find_by_email: function(string): (User, string)
    end
    return users
`)

env.PreloadModule("users", func(L *glua.LState) int {
    mod := L.SetFuncs(L.NewTable(), map[string]glua.LGFunction{
        "count": func(L *glua.LState) int {
            n, err := usersService.Count(context.Background())
            if err != nil {
                L.Push(glua.LNil); L.Push(glua.LString(err.Error()))
                return 2
            }
            L.Push(glua.LNumber(n)); L.Push(glua.LNil)
            return 2
        },
        // ...
    })
    L.Push(mod)
    return 1
})
```

Then in the REPL:

```
myapp(teal)> local n, err = users.count()
myapp(teal)> print("users:", n)
```

`SetType` is optional but strongly recommended — without it, Teal
errors on `require("users")` and users have to drop to `--mode lua`.

## Configuration

`config/repl.toml` — scaffolded, edit as needed:

```toml
mode = "teal"   # or "lua"
```

`--mode teal|lua` flag on the command overrides the config.

## Production caveat

In `env.Production`, the REPL prints a banner:

```
⚠  connected to PRODUCTION — writes are real.
```

That's the only guardrail. There's no dry-run mode. Every write in
the REPL is a real write on the live app's DB/cache/queue. Use
sparingly, and consider running against a read replica when the
option exists.

## Limitations & follow-ups

- **No readline** — `bufio.Scanner` reads plain lines. Arrow-key
  history, line editing, and tab completion require a readline
  library (chzyer/readline). Follow-up: pi-fox.6.
- **No multi-line continuation** — function definitions must be
  one-liners or come from a script file. Follow-up: pi-fox.6.
- **No dot-commands** — `.help`, `.mode`, `.env`, `.reset` are on
  the roadmap. Follow-up: pi-fox.6.
- **Events/queue subscribe from Lua** — v1 is emit/publish-only.
  Cross-goroutine callbacks into an LState need serialisation
  work. Follow-up: separate beads task when needed.
