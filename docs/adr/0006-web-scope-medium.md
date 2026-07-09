# hex/web is medium-scope: server + standard middleware

`hex/web` wraps `labstack/echo` and ships:

- an `echo.Echo` service exposed via a provider,
- hex-standard middleware (request ID, structured logging, panic recovery, CORS),
- health/readiness endpoints,
- graceful shutdown wired to `app.Shutdown` via the `Shutdowner` interface.

Consumers register their own routes, handlers, and additional middleware. hex does not scaffold a controller/router directory convention (that stays app-specific — the bot's `web/controller/<name>/get.go` layout is one valid convention, not the only one).

We rejected the "thin" alternative because every hex service needs the same middleware stack; making it opt-in guarantees drift. We rejected "thick" because the moment hex prescribes routing conventions, we lock in one app's shape and make hex less attractive to future consumers with different HTTP shapes.
