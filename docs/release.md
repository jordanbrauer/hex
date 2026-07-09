# Releasing hex

The release pipeline ships **the `hex` CLI scaffolder only**. The rest
of the repo is a Go module — consumers import it via `go get` and
resolve versions from git tags directly, no build artefacts required.
Every tag serves both purposes: it's what `go get github.com/
jordanbrauer/hex@vX.Y.Z` resolves against, *and* it's what fires the
GoReleaser workflow that publishes the CLI binaries.

Publishing a GitHub Release with a semver tag triggers
`.github/workflows/release.yml`, which runs
[GoReleaser](https://goreleaser.com) against `.goreleaser.yaml`:

1. Cross-compiles the `hex` CLI (`./cmd/hex`) for darwin + linux,
   amd64 + arm64.
2. Packages each build as a `.tar.gz` archive with `checksums.txt`.
3. Uploads archives + checksums as release assets.
4. Regenerates `hex.rb` at the repo root pointing at the new
   archives and commits it back to `main`. Homebrew's tap loader
   accepts formulas at `./`, `Formula/`, or `HomebrewFormula/`;
   hex uses the root path for a flatter layout.

## Cutting a release

Every release is a git tag + GitHub Release pair.

```sh
# 1. Make sure main is green.
just check

# 2. Tag on main.
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0

# 3. Draft the release on GitHub using the pushed tag, then publish.
gh release create v0.1.0 --generate-notes
```

Publishing the release fires the workflow. The workflow finishes with a
new commit on `main` of the form `brew: update hex to v0.1.0` — that's
the formula bump.

## Verifying locally before you tag

Dry-run the whole pipeline without touching the tag or pushing anything:

```sh
goreleaser release --snapshot --clean --skip=publish
ls ./dist
```

Check the archive names match `hex_<version>_<os>_<arch>.tar.gz`, the
per-platform binaries run (`./dist/hex_darwin_arm64_v0.0.0-…/hex --help`),
and the generated formula under `./dist/homebrew/` looks reasonable.

## macOS Gatekeeper

hex ships **unsigned**. The release archives are `.tar.gz` and the
Homebrew formula uses `brews:` (Formula) — not `homebrew_casks:` (Cask)
— because Homebrew's formula install path extracts the tarball with
`tar`, which drops the `com.apple.quarantine` extended attribute the
browser adds to downloads. Users install with zero Gatekeeper prompts,
no self-signing, no notarisation.

If hex CLI ever ships codesigned + notarised binaries, migrate the
release pipeline to `homebrew_casks:` in `.goreleaser.yaml` and drop
this workaround.

## Installation, once released

```sh
brew tap jordanbrauer/hex https://github.com/jordanbrauer/hex
brew install jordanbrauer/hex/hex

# Verify.
hex --help
```

Upgrading:

```sh
brew update
brew upgrade hex
```
