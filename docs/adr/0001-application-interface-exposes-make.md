# Application interface exposes Make() during all lifecycle phases

The `hex.Application` interface includes `Make()` alongside `Bind()` and `Singleton()`, making it available during both `Register()` and `Boot()` phases. This is intentional — some providers need to resolve early primitives (e.g., config) during registration to decide what to bind. The bootstrap ordering (providers registered first are booted first) is the mechanism that makes this safe. This is a "know what you're doing" capability: resolving a binding that hasn't been registered yet returns a clear error, but the framework does not prevent it at the type level.

We considered splitting the interface so `Register()` gets a write-only surface and `Boot()` gets the full surface. Rejected because both finch-cli and finch-bot have operated fine with the unified interface for their entire lifetime, and the split adds complexity for a problem that hasn't materialized.
