# NOTICE — Fennel compiler

This directory contains a vendored copy of the Fennel compiler
amalgamation:

- **File**: `fennel.lua`
- **Version**: 1.6.1
- **Upstream**: https://fennel-lang.org / https://github.com/bakpakin/Fennel
- **License**: MIT (see the SPDX header at the top of the file)
- **Copyright**: Calvin Rose and Fennel contributors

The file is used verbatim, unmodified, embedded via `//go:embed` and
loaded into gopher-lua at runtime by the `Load` function in this
package.

## Why vendored

Fennel ships as a single amalgamated Lua source file that embeds
the parser, compiler, macro engine, and REPL. Distributing hex with
this file (rather than downloading it at build time or asking users
to install `fennel` themselves) keeps the framework hermetic and
lets `hex init` produce a fully-working `.fnl`-capable app with no
extra installation steps.

## Updating

To upgrade to a newer Fennel release:

1. Download the release amalgamation from
   https://fennel-lang.org/downloads (typically
   `fennel-<version>.lua`).
2. Replace `fennel.lua` in this directory with the new file.
3. Update the version noted above.
4. Run `go test ./lua/fennel/... ./lua/repl/...` and the end-to-end
   REPL smoke checks.
5. Bump the doc comment at the top of `fennel.go` if the API surface
   changed (unlikely for point releases).

## Attribution

`fennel.lua` is distributed under the MIT License. The full license
text is in [`LICENSE`](./LICENSE) alongside this NOTICE, and the SPDX
header at the top of `fennel.lua` records the copyright.

If you build against or redistribute hex including this package, the
MIT terms in `LICENSE` apply to `fennel.lua` in addition to hex's own
license.
