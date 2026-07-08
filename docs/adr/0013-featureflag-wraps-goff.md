# hex/featureflag wraps go-feature-flag

`hex/featureflag` wraps github.com/thomaspoignant/go-feature-flag (GOFF). GOFF speaks OpenFeature semantics, supports rule-based targeting, percentage rollouts, progressive rollouts, and comes with retrievers for file, HTTP, S3, GitHub, GitLab, Kubernetes, Postgres, Redis, and more. It also ships notifiers for flag-change hooks. Rolling our own would either miss most of that or turn into a many-thousand-line project.

The wrapper follows the pattern established for cron/log/web/lua/pool/policy/casbin/i18n: type aliases for `Client`, `Context`, and `EvaluationContext` so consumers get the full GOFF API through the aliases, hex-owned constructors that accept a flag file from disk or `fs.FS`, and a small package-level convenience layer (`SetDefault` + `Bool`/`Int`/`String`/`Float64`/`JSON`).

## v1 retrievers

- **file** — GOFF's built-in file retriever. Path-based, polls for changes at PollingInterval.
- **embed.FS** — hex-owned retriever satisfying GOFF's `Retriever` interface. Reads a flag file from a `fs.FS`, immutable at build time. Useful for shipping baseline flags with the binary.

Other retrievers (HTTP, S3, K8s ConfigMap, Postgres, Redis) are already available upstream; consumers who need them import GOFF's retriever subpackage directly and pass it through hex/featureflag's Options.

## Test fixtures are files

Same rule as hex/i18n and hex/policy: YAML flag definitions live under `testdata/` and are embedded via `//go:embed`. YAML with rule DSL loses too much structure as string literals, and the whole point of GOFF is that flag config is data operators can edit.

## Not shipped

- Notifiers (webhook/Slack/logs on flag change) are consumer-side. If a consumer wires them they use GOFF's `notifier` subpackage directly through hex/featureflag's Options.
- OpenFeature provider wrapper — GOFF has one already (`gofeatureflag/openfeature`). Consumers who want OpenFeature semantics can layer that on top; hex/featureflag stays close to GOFF's native surface for now.
