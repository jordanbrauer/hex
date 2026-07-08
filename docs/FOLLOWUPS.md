# Follow-ups

Subsystems not built yet, sorted by category. Decisions and ADRs land as each is picked up.

## Deferred medium subsystems

| Package | What | Notes |
|---|---|---|
| `hex/session` | Server-side sessions (cookie-encrypted + store-backed) | Distinct from cache; adds crypto + expiry semantics. Store adapters: memory, sqlite, redis. |
| `hex/mail` | Templated email with pluggable transports | Wraps `wneessen/go-mail`. SMTP, SES, sendgrid drivers as opt-in subpackages. |
| `hex/notify` | Multi-channel notification dispatcher (email/Slack/Discord/webhook) | Laravel Notifications equivalent. Composes hex/mail + hex/queue. |
| `hex/webhook` | Outbound HMAC-signed webhook delivery + inbound signature verification | Composes hex/queue (retry) + hex/hash (HMAC). |
| `hex/search` | Fulltext search abstraction | v1: sqlite FTS. Later: meilisearch, elasticsearch driver subpackages. |

## Web-adjacent (probably folds into hex/web)

- CSRF middleware
- Rate-limit middleware (composes hex/ratelimit)
- JWT verify middleware
- Static file serving with SPA fallback
- Templating layout helpers (html/template extends/blocks)
- Assets bundling (vite embed)

## Auth (deferred)

Authentication counterpart to hex/policy. Land only if consumer apps prove they need shared primitives here rather than owning it.

- `hex/token` — JWT sign/verify, opaque bearer tokens, refresh flows
- `hex/oauth` — thin wrapper around `golang.org/x/oauth2` with provider registry
- `hex/password` — argon2id hashing (may fold into hex/hash — already in essentials)

## Config — Laravel-style source prefix routing

Allow reads like `config.String("hex:database.dsn")` to return the value contributed by a *specific* source layer rather than the merged/effective value. Modelled after Laravel's vendor vs. published-config distinction.

**Sketch:**

```go
type Source struct {
    Name string   // "hex", "app", or vendor label like "stripe"
    FS   fs.FS
}

type Config struct {
    Sources []Source   // replaces current []fs.FS
    // ...
}
```

**Read semantics:**

```go
config.String("database.dsn")         // merged/effective (current behavior)
config.String("hex:database.dsn")     // framework layer only
config.String("app:database.dsn")     // consumer override layer only
config.String("stripe:payments.key")  // third-party layer only
```

**Convention:**

| Prefix | Owner | Assigned by |
|---|---|---|
| `hex` | official hex framework providers | scaffolder (hardcoded) |
| `app` | consumer's own `config/` dir | scaffolder (hardcoded) |
| `<vendor>` | third-party packages | consumer picks in their factory |

Reserved names are convention-only — not enforced.

**Storage:** `Store` keeps a merged view plus per-source-name views. Prefix parser splits on the first colon, routes to the per-source view for that name; no prefix routes to the merged view. Unknown prefix returns the zero value silently.

**Third-party integration:** packages expose `Configs() fs.FS` and recommend a name in their docstring; consumer adds `{Name: "...", FS: pkg.Configs()}` to their sources. A future `hex install <package>` command would automate this wiring.

**Colon-in-value hazard:** none — this is *lookup-key* parsing, not value parsing. Values like `postgres://user:pass@host` are read verbatim.

**Namespace collisions between third-parties:** later Source wins in merged view; document as a known hazard, don't enforce.

**Deferred because:** the immediate need is framework providers publishing their own configs into consumer apps; source-prefix reads are a diagnostic/introspection nicety on top of that. Land after the publish flow is proven.

## Deferrable (opinionated / low reuse)

- PDF generation
- Payment integration (Stripe/Square)
- Full CMS/admin scaffolding
- WebSockets abstraction
- gRPC
- Testcontainers helpers
- Secrets management (Vault/AWS Secrets Manager)
