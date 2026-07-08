# ADR 0016 — Runtime Environment is a first-class hex concern

**Status:** Accepted
**Date:** 2026-07-08

## Context

Discovered while investigating [pi-6u2](https://github.com/jordanbrauer/hex/issues) (the finch-cli bug where `app.New()` in tests silently opened the real on-disk SQLite DB): tests, dev runs, and released binaries all boot the same code path but need different drivers. The traditional finch fix would have been a per-project `testutil.NewTestApp(t)` that rebinds specific providers. But this only helps consumers who know to use it, and the trap is silent.

We wanted something better: bake the notion of "what environment am I running in" into the framework so:

- Every provider has access to it without ad-hoc env-var reading.
- Config files can carry per-env overrides declaratively.
- Tests default to a safe environment automatically.
- Production is unchanged and doesn't pay a discovery cost.

## Decision

Three orthogonal changes, all landing together:

### 1. `hex/env` package with strict enum

```go
type Environment string

const (
    Development Environment = "development"
    Test        Environment = "test"
    Production  Environment = "production"
)
```

- **Strict enum.** No staging. Staging is a deployment concern, not a runtime concern — apps typically look identical in staging and production apart from a `deploy.tier` config key or similar.
- **Detection order:** `HEX_ENV` → `APP_ENV` → `testing.Testing()` (Go 1.21+) → Development.
- **`Parse()` accepts aliases** (`dev`, `local`, `ci`, `testing`, `prod`, `live`) and returns an error for unknown strings.

### 2. `hex.App.Environment()` accessor + `provider.Application` interface

`hex.New(hex.WithEnvironment(env.X))` sets it explicitly; auto-detect fills in when not set. The `provider.Application` interface gains an `Environment() env.Environment` method so any provider can consult it during Register/Boot without reaching around.

### 3. Config layering per environment

`hex/config` grew a `Config.Environment` field. When set, files named `<namespace>.<env>.toml` overlay their base `<namespace>.toml` counterpart in the same source. Overlays merge over the base per namespace; sources still merge in registration order (framework → app → …).

Example directory:

```
config/
├── database.toml               # base defaults
├── database.test.toml          # dsn=":memory:", 1 conn
├── database.production.toml    # postgres dsn, tuned pool
├── cache.toml                  # driver="memory" size=100
└── cache.test.toml             # size=5 for hermetic tests
```

## Alternatives considered

1. **Consumer-side `testutil.NewTestApp(t)` only** — solves the immediate problem but doesn't scale beyond tests, doesn't help third-party providers, and the trap remains silent.
2. **Env-guarded refusal in db/provider** — refuse to open a real DB path unless `HEX_ALLOW_REAL_DB=1`. Rejected because it puts the safety mechanism in one provider instead of at the framework level.
3. **Environment as a config value only** (no first-class type) — a namespaced key like `hex.env = "test"`. Rejected because it's discovered too late (after config loads), and providers would still need helpers to read + parse it. Not worth the indirection.
4. **Subdirectory-based overlays** (`config/environments/test/database.toml`) — considered and preferred by some frameworks. Rejected here because:
   - Sub-extension (`database.test.toml`) is a single-file lookup with no nesting.
   - `//go:embed *.toml` picks them up automatically without new glob patterns.
   - Aligns with the naming convention already used for CUE (`database.cue`) and the app's own schema stub (`schema.cue`).
5. **Include a `Staging` constant** — rejected as unnecessary. Staging is where you deploy production code with a `deploy.tier=staging` config value.

## Consequences

**Positive:**
- Provider authors get env-aware wiring for free without importing hex-root or reading env vars.
- Consumers get declarative per-env overrides via config files — same code, same providers, different drivers.
- Tests default to `Test` env automatically through `testing.Testing()`, so accidental real-DB usage requires an explicit opt-in.
- `hextest.NewApp(t, providers...)` gives a one-liner test bootstrap that works with any provider set.
- Environment becomes a debug knob: `HEX_ENV=production go run .` boots a dev binary as if it were production, useful for local repro of prod-only issues.

**Negative:**
- Every provider that implements the `provider.Application` interface manually (mocks in tests) must now satisfy `Environment()`. Trivially fixed with a one-line method, but it's a tiny break for existing test doubles.
- The Config type grew a new field; consumers who instantiated it directly (rather than through the provider) may want to set `Environment` explicitly.
- Config file layout has a new pattern (`<ns>.<env>.toml`) to learn. Documented on the README and in the scaffolder templates.

**Neutral:**
- Staging deployments still need somewhere to name themselves. Convention: a config key like `deploy.tier = "staging" | "production"` in `config/*.production.toml`. Not enforced by hex.

## References

- `hex/env/env.go` — Environment type + Detect
- `hex/app.go` — `App.Environment()` + `WithEnvironment` option
- `hex/config/config.go` — overlay loading in `Load`
- `hex/config/provider/provider.go` — passes `app.Environment()` through
- `hex/hextest/hextest.go` — test bootstrap helper
- Delivered commit: `00ee1a2 hex/env: runtime environment awareness + config overlays`
- Closes: pi-6u2
