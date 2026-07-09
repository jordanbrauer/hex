# Third-party notices

hex is MIT-licensed (see [`LICENSE`](./LICENSE)). It embeds source
from several third-party projects that carry their own license terms.
Each vendored directory has both a `NOTICE.md` explaining what's there
and one or more `LICENSE*` files reproducing the upstream license
text verbatim.

If you redistribute hex (as source, as a binary, or as a container
image), you must reproduce these notices and licenses too.

## Vendored source

| Path | Upstream | License |
|---|---|---|
| [`lua/fennel/fennel.lua`](./lua/fennel/fennel.lua) | [Fennel](https://github.com/bakpakin/Fennel), v1.6.1 | MIT — [`lua/fennel/LICENSE`](./lua/fennel/LICENSE) |
| [`lua/teal/tl.lua`](./lua/teal/tl.lua) | [Teal](https://github.com/teal-language/tl), v0.13.2+dev | MIT — [`lua/teal/LICENSE-tl`](./lua/teal/LICENSE-tl) |
| [`lua/teal/compat52.lua`](./lua/teal/compat52.lua) | [algernon](https://github.com/xyproto/algernon) (patched for gopher-lua) | BSD-3-Clause — [`lua/teal/LICENSE-algernon`](./lua/teal/LICENSE-algernon) |
| [`lua/teal/bit.lua`](./lua/teal/bit.lua) | [algernon](https://github.com/xyproto/algernon) (patched for gopher-lua) | BSD-3-Clause — [`lua/teal/LICENSE-algernon`](./lua/teal/LICENSE-algernon) |
| [`lua/teal/bit32.lua`](./lua/teal/bit32.lua) | [algernon](https://github.com/xyproto/algernon) (patched for gopher-lua) | BSD-3-Clause — [`lua/teal/LICENSE-algernon`](./lua/teal/LICENSE-algernon) |

## Go module dependencies

The Go module dependencies declared in [`go.mod`](./go.mod) are linked
dynamically at build time by the Go toolchain — hex neither vendors
nor redistributes their source. Their licenses apply to the compiled
binaries hex produces (e.g. the `hex` CLI shipped via Homebrew or
`go install`); consult each dependency's upstream repository for the
authoritative license.

The wrapped-library table in [`AGENTS.md`](./AGENTS.md#conventions)
lists every notable direct dependency and the ADR that motivated
wrapping it. Notable direct dependencies include (non-exhaustive):

- [`labstack/echo`](https://github.com/labstack/echo) — MIT
- [`spf13/cobra`](https://github.com/spf13/cobra) — Apache-2.0
- [`spf13/viper`](https://github.com/spf13/viper) — MIT
- [`charmbracelet/log`](https://github.com/charmbracelet/log) — MIT
- [`charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss) — MIT
- [`charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) — MIT
- [`yuin/gopher-lua`](https://github.com/yuin/gopher-lua) — MIT
- [`yuin/goldmark`](https://github.com/yuin/goldmark) — MIT
- [`Joker/jade`](https://github.com/Joker/jade) — MIT
- [`casbin/casbin`](https://github.com/casbin/casbin) — Apache-2.0
- [`leonelquinteros/gotext`](https://github.com/leonelquinteros/gotext) — MIT
- [`thomaspoignant/go-feature-flag`](https://github.com/thomaspoignant/go-feature-flag) — MIT
- [`alitto/pond`](https://github.com/alitto/pond) — Apache-2.0
- [`robfig/cron`](https://github.com/robfig/cron) — MIT
- [`golang-migrate/migrate`](https://github.com/golang-migrate/migrate) — MIT
- [`cuelang.org/go`](https://cuelang.org) — Apache-2.0
- [`go-bdd/gobdd`](https://github.com/go-bdd/gobdd) — MIT
- [`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite) — BSD-3-Clause
- [`PuerkitoBio/goquery`](https://github.com/PuerkitoBio/goquery) — BSD-3-Clause

Run `go mod graph` for the full transitive list.
