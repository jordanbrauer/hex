Scaffold a new hex application in the given directory (or the current one).

Run without a name to scaffold into `.`; otherwise a subdirectory named `<name>`
is created. Interactive prompts fill in the Go module path, the binary name, the
database dialect, and which framework components to enable. Pass `--yes` to skip
the prompts and take the flag defaults instead.

The generated project is runnable immediately: it wires an app kernel, a
provider registry, config, logging, and an embedded Lua REPL, and drops the
`// hex:*` marker comments that `hex make` uses to auto-wire new code.

Developer tooling — `.editorconfig`, a `.golangci.yml` lint config, and a
`.goreleaser.yaml` release config — is scaffolded by default. Opt out with
`--editorconfig=false`, `--lint=false`, or `--goreleaser=false`. Pass `--github`
to also scaffold a `.github/workflows/release.yml` that runs goreleaser on
tag push (implies `--goreleaser`).
