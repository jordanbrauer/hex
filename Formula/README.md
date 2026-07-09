# Formula/

This directory holds the Homebrew formula for the `hex` CLI. It's
generated and committed by [GoReleaser](https://goreleaser.com) as part
of the release workflow (`.github/workflows/release.yml`), not written
by hand.

The formula lives in this repository — not in a separate
`homebrew-tap` repo — so users install by tapping the source repository
directly:

```sh
brew tap jordanbrauer/hex https://github.com/jordanbrauer/hex
brew install jordanbrauer/hex/hex
```

## Why formula, not cask?

hex ships unsigned. The formula install path extracts the release
tarball with `tar`, which drops the `com.apple.quarantine` extended
attribute the browser adds to downloads. Casks download raw binaries
and preserve the attribute, so casks would trigger Gatekeeper prompts
on every install and update.

If hex CLI ever ships codesigned + notarised binaries, migrate the
release pipeline to `homebrew_casks:` in `.goreleaser.yaml`.
