# ADR 0016 — Runtime Environment is a first-class hex concern

**Status:** Accepted
**Date:** 2026-07-08

## Context

Discovered while investigating a bug where `app.New()` in tests silently opened the real on-disk SQLite DB: tests, dev runs, and released binaries all boot the same code path but need different drivers. The traditional fix would have been a per-project `testutil.NewTestApp(t)` that rebinds specific providers. But this only helps consumers who know to use it, and the trap is silent.

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

### 3. Env-specific overrides via `.env.<env>`

`hex/config` grew a `Config.Environment` field. When set (and `EnvFile` is populated), Load also attempts to load `<EnvFile>.<Environment>` before the base `.env`. godotenv's no-overwrite semantics give the desired precedence:

```
OS env  >  .env.<env>  >  .env  >  base TOML in config/
```

Env bindings declared in `env.yaml` map config keys to env-var names, and CUE schema validation runs against the merged view including env-bound values, so bad env overrides fail loudly at Load time just like bad TOML values would.

Example layout:

```
.env                 # local defaults, typically gitignored
.env.test            # committed; test overrides (DSN=:memory:, small pools)
.env.production      # committed; production overrides (postgres DSN, tuned pool)
config/
├── env.yaml         # binding: config.key -> ENV_VAR
├── database.toml    # dev defaults
└── cache.toml       # dev defaults
```

## Alternatives considered

1. **Consumer-side `testutil.NewTestApp(t)` only** — solves the immediate problem but doesn't scale beyond tests, doesn't help third-party providers, and the trap remains silent.
2. **Env-guarded refusal in db/provider** — refuse to open a real DB path unless `HEX_ALLOW_REAL_DB=1`. Rejected because it puts the safety mechanism in one provider instead of at the framework level.
3. **Environment as a config value only** (no first-class type) — a namespaced key like `hex.env = "test"`. Rejected because it's discovered too late (after config loads), and providers would still need helpers to read + parse it. Not worth the indirection.
4. **TOML overlay files** (`config/database.test.toml` overlays `config/database.toml`) — initially implemented in commit `00ee1a2` and reverted in favour of `.env.<env>`. Rejected because:
   - Introduced a second override mechanism alongside env vars, doubling the surface consumers had to learn.
   - env.yaml already exists as an explicit contract of overridable keys; `.env.<env>` reuses it verbatim.
   - `.env.<env>` matches the ecosystem pattern (Node/Rails/Laravel) instead of a hex-specific TOML sub-extension.
   - CUE validation covers env-bound values equally well (viper's `AllSettings` evaluates env bindings at Load, so bad env values fail schema validation same as bad TOML values).
5. **Subdirectory-based overlays** (`config/environments/test/database.toml`) — considered and dropped alongside TOML overlays for the same reasons.
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
