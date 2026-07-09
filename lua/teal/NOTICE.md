# hex/lua/teal — vendored assets

This package embeds four Lua files that make [Teal](https://github.com/teal-language/tl)
runnable inside [gopher-lua](https://github.com/yuin/gopher-lua).

## Files

| File            | Upstream                                       | License                          |
|-----------------|------------------------------------------------|----------------------------------|
| `tl.lua`        | Teal compiler v0.13.2+dev                      | MIT                              |
| `compat52.lua`  | Lua 5.2 compatibility shim (patched)           | MIT                              |
| `bit.lua`       | LuaJIT `bit` compatibility (patched)           | MIT                              |
| `bit32.lua`     | Lua 5.2 `bit32` compatibility (patched)        | MIT                              |

`compat52.lua`, `bit.lua`, and `bit32.lua` carry small patches applied by
[xyproto/algernon](https://github.com/xyproto/algernon) so that gopher-lua
(which implements Lua 5.1 semantics) can run the Teal compiler unchanged.
The patches are documented in
[yuin/gopher-lua#314](https://github.com/yuin/gopher-lua/issues/314).

hex vendors the patched versions from algernon (BSD-3-Clause). The Teal
compiler itself is unmodified.

## Updating

`tl.lua` is currently pinned to Teal v0.13.2+dev (matches algernon's
pin). To update, fetch a newer release from teal-language/tl and
overwrite `tl.lua`. If newer Teal versions rely on Lua semantics not
covered by the compat shims, additional patches may be needed.
