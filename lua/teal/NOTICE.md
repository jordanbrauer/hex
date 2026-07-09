# hex/lua/teal — vendored assets

This package embeds four Lua files that make [Teal](https://github.com/teal-language/tl)
runnable inside [gopher-lua](https://github.com/yuin/gopher-lua). All
files are unmodified relative to their upstream sources except where
noted; nothing in this directory is hex-authored.

## Files

| File            | Upstream                                                          | License                                 |
|-----------------|-------------------------------------------------------------------|-----------------------------------------|
| `tl.lua`        | [teal-language/tl](https://github.com/teal-language/tl), v0.13.2+dev | MIT — see [`LICENSE-tl`](./LICENSE-tl)  |
| `compat52.lua`  | Lua 5.2 compat shim, patched by algernon for gopher-lua           | BSD-3-Clause — see [`LICENSE-algernon`](./LICENSE-algernon) |
| `bit.lua`       | LuaJIT `bit` compat, patched by algernon for gopher-lua           | BSD-3-Clause — see [`LICENSE-algernon`](./LICENSE-algernon) |
| `bit32.lua`     | Lua 5.2 `bit32` compat, patched by algernon for gopher-lua        | BSD-3-Clause — see [`LICENSE-algernon`](./LICENSE-algernon) |

`compat52.lua`, `bit.lua`, and `bit32.lua` come from
[xyproto/algernon](https://github.com/xyproto/algernon) with small
patches applied so that gopher-lua (which implements Lua 5.1 semantics)
can run the Teal compiler unchanged. The patches are documented in
[yuin/gopher-lua#314](https://github.com/yuin/gopher-lua/issues/314).
The three shim files are distributed under algernon's BSD-3-Clause
terms as shipped. `tl.lua` itself is unmodified and covered by the
Teal MIT license.

## Attribution

The two license texts alongside this NOTICE apply to `tl.lua` and the
three compat shims respectively:

- [`LICENSE-tl`](./LICENSE-tl) — Teal (© 2019 Hisham Muhammad, MIT)
- [`LICENSE-algernon`](./LICENSE-algernon) — algernon (© Alexander F.
  Rødseth, BSD-3-Clause)

If you build against or redistribute hex including this package, both
sets of terms apply to the corresponding files in addition to hex's
own license.

## Updating

`tl.lua` is currently pinned to Teal v0.13.2+dev (matches algernon's
pin). To update, fetch a newer release from teal-language/tl and
overwrite `tl.lua`. If newer Teal versions rely on Lua semantics not
covered by the compat shims, additional patches may be needed. If you
also re-vendor the compat shims from a newer algernon revision,
refresh [`LICENSE-algernon`](./LICENSE-algernon) at the same time.
