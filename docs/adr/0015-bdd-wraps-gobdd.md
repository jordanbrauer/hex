# hex/bdd wraps gobdd

`hex/bdd` wraps github.com/go-bdd/gobdd — a BDD test runner for Go that reads standard Gherkin `.feature` files and matches step definitions to expressions. Same pattern as our other wraps (cron, log, web, casbin, gotext, gobdd): type aliases for the upstream types, hex-owned constructors, and one embed.FS convenience so consumers can ship feature files inside binaries.

## Scope

- Type aliases: `Suite`, `SuiteOptions`, `Context`, `TestingT`, `StepTest`.
- Option re-exports so consumers do not need to import gobdd directly for typical usage: `RunInParallel`, `WithFeaturesPath`, `WithTags`, `WithIgnoredTags`, `WithBefore/After Scenario/Step`.
- Constructors: `NewSuite(t, opts...)` for the standard on-disk shape, and `NewSuiteFS(t, fs, glob, opts...)` for consumers who `//go:embed` feature files.
- The FS variant materialises features into a temp directory at test time. gobdd's feature loader is filesystem-path oriented and its `featureSource` interface is unexported, so extracting to a `t.TempDir()` is the pragmatic bridge until upstream exposes a reader/FS-based source.

## Meta note

hex itself does not adopt gobdd for its own tests. Its own tests remain standard `go test`. hex/bdd is a library hex applications may choose to use — same relationship as hex/lua (framework ships the runtime, consumers use it or not).
