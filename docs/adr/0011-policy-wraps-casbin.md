# hex/policy wraps Casbin

`hex/policy` is a thin wrapper around github.com/casbin/casbin/v2. Casbin's model config DSL (request definition, policy definition, matchers, effect) supports ACL, RBAC, ABAC, and hybrid models from one engine, and it lets us swap policy storage (memory, file, SQL) via an adapter interface.

The wrapper follows the pattern established for cron/log/web/lua/pool: type alias `*casbin.Enforcer` + hex-owned constructors that accept a model (string / embed.FS / path) and an adapter. Consumers get the full Casbin API through the alias with no re-export layer.

## v1 adapters

- **memory** — Casbin's built-in in-memory adapter. For tests and single-node deployments where policies are loaded at startup.
- **file** — Casbin's built-in CSV file adapter. For dev workflows and Git-versioned policy sets.
- **sql** deferred — landing when a consumer needs it, following the `hex/db/sqlite` + `hex/db/postgres` sub-package pattern. The plan is to wrap `github.com/casbin/sql-adapter` (raw database/sql, no gorm) so hex/policy inherits hex/db's driver-agnostic story.

## Test fixtures are files, not string literals

Model .conf and policy .csv files are checked in as real files under `testdata/` and embedded via `embed.FS`. Inline string literals lose the DSL's tabular structure and defeat the point of having a file-based model.

## Not the same as authentication

hex/policy is authorisation: "given identity X, may they perform action Y on resource Z?". Authentication (proving X is who they say they are — OAuth, sessions, tokens) stays in consumer apps because it is provider-specific.
