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

## Deferrable (opinionated / low reuse)

- PDF generation
- Payment integration (Stripe/Square)
- Full CMS/admin scaffolding
- WebSockets abstraction
- gRPC
- Testcontainers helpers
- Secrets management (Vault/AWS Secrets Manager)
