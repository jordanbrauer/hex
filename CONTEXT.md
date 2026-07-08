# hex

A Go application framework that provides the foundational building blocks — IoC container, service providers, event bus, config, database, logging — and a scaffolding CLI to generate projects and code following hex conventions.

## Language

**App**:
The application kernel. Owns the container, provider registry, and event bus. One per process.
_Avoid_: Kernel, runtime

**Container**:
The IoC dependency injection container. Holds named bindings (factories) and resolves them by name with type-safe generics.
_Avoid_: Registry (when referring to DI), injector, locator

**Binding**:
A named factory registered in the container. Either transient (new instance per resolution) or singleton (cached after first resolution).
_Avoid_: Registration, entry

**Provider**:
A service provider — a struct that registers bindings into the container during startup. Lifecycle: Register → Boot → Shutdown.
_Avoid_: Module, plugin, bundle

**Registry**:
The ordered collection of providers that the app bootstraps in sequence. Not the container — the provider list.
_Avoid_: Provider list, provider set

**Bootstrap**:
The startup sequence: Register all providers, then Boot all providers, in registration order. Providers registered first are booted first.
_Avoid_: Init, startup, setup

**Application**:
The interface that providers interact with during lifecycle hooks. Exposes container methods (Bind, Singleton, Make) and event bus methods (On, Emit). Satisfied by `*App`.
_Avoid_: Context, kernel interface

**Generator**:
A `hex make:*` CLI command that produces correctly-placed source files following hex conventions.
_Avoid_: Scaffold (when referring to a single file), template (that's what generators use internally)

**Scaffold**:
The full project structure created by `hex init`. A scaffold is a complete, runnable project.
_Avoid_: Skeleton, boilerplate, starter

**Env Map**:
A YAML file (`config/env.yaml`) that declaratively maps config keys to environment variable names. Not app config — it's a binding declaration that tells Viper which env vars override which config keys.
_Avoid_: Env config, environment file (that's `.env`)

**Disk**:
A named backend for reading and writing files (local filesystem, S3, MinIO, GCS). A hex app can have several disks configured concurrently and address each by name (`disk.Get("uploads")`). Interface lives in `hex/disk`; concrete backends live in subpackages.
_Avoid_: Storage, Bucket, Filesystem (the last is what the local backend wraps, not the abstraction).

**Cache**:
A named key-value store with TTL semantics. Backends (memory, redis, valkey, memcached) implement the same `Cache` interface. Consumers resolve caches by name from the container.
_Avoid_: Store (that's config), KV, Session (which is app-specific state).

**Job** (cron):
A named, scheduled unit of work registered with the `hex/cron` scheduler. Jobs have a cron expression and a run function; the scheduler owns tick timing and lifecycle. Not the same as **Job** (queue) or **Task** (pool) — all three are units of work but they differ in trigger and semantics.
_Avoid_: Task (too generic), Cron (that's the whole scheduler subsystem).

**Pool**:
A bounded worker pool for running tasks concurrently in-process. Distinct from a **Queue** (no delivery, no persistence, in-memory only) and from **Cron** (no schedule). Backed by alitto/pond.
_Avoid_: Pool alone can mean database connection pool; refer to those by their concrete type (`*sql.DB`).

**Tracer** (telemetry):
An OpenTelemetry tracer that emits spans for a bounded unit of work (usually one instrumented library, package, or component). Consumers request tracers by name from the provider hex/telemetry sets up.
_Avoid_: Trace (a trace is a tree of spans, not the emitter).

**Meter** (telemetry):
An OpenTelemetry meter that emits metrics (counters, histograms, gauges). Analogue of Tracer for metrics; consumers request meters by name.
_Avoid_: Metric (that is the emitted value), Recorder.

**Flag** (featureflag):
A named boolean/int/string/float/JSON value evaluated against an EvaluationContext. Flag config lives in a data file (YAML/JSON/TOML) loaded by a **Retriever**. Different from a CLI flag (`--verbose`) — when both concepts appear together, disambiguate with "feature flag" and "CLI flag."
_Avoid_: Toggle (some tools call these toggles), Switch.

**Retriever** (featureflag):
The source of flag definitions — file, embed.FS, HTTP, S3, K8s ConfigMap, etc. Different from **Adapter** (policy) even though both are pluggable storage: retrievers are read-only and polling-based, adapters are read/write and event-driven.
_Avoid_: Loader, Source.

**Locale** (i18n):
A language-scoped bundle of translations backed by one or more PO files. Loaded from disk or `fs.FS`. Different from a **Model** (policy) even though both hold configuration — a locale carries translations, a model carries authorisation DSL.
_Avoid_: Language (means the ISO code, not the loaded bundle), Bundle.

**Domain** (i18n):
A PO file's logical grouping (e.g. `messages`, `errors`, `emails`). One Locale can hold multiple domains, addressed by name at lookup time.
_Avoid_: Namespace (used for config), Scope.

**Translator** (i18n):
A hex-owned container that holds several **Locale**s and picks one per call based on a language code the caller supplies. Not the same as gotext's `Translator` interface (which represents a single locale) — hex/i18n's `Translator` is a multi-locale layer above it.
_Avoid_: I18n, TranslationSet.

**Model** (policy):
A Casbin model config — the DSL file (`.conf`) that declares request/policy shape, role definitions, matchers, and effect. Models are static (loaded at startup); policies are the dynamic rules evaluated against a model.
_Avoid_: Schema, Config (used for `hex/config`).

**Policy** (policy):
A rule row evaluated by an Enforcer against a Model, e.g. `p, alice, data1, read`. Policies live in an Adapter (memory, CSV file, SQL table). Not the same as a **Model** — the model defines the language, policies fill in the rules.
_Avoid_: Rule (used for validation elsewhere), Grant.

**Adapter** (policy):
The storage backend for policies. hex/policy ships memory + file adapters; SQL lands later via a subpackage. Different from **Provider** (framework-level lifecycle).
_Avoid_: Store, Backend.

**Enforcer** (policy):
A runtime Casbin engine bound to a Model and an Adapter. `enforcer.Enforce(sub, obj, act)` returns whether the tuple is permitted.
_Avoid_: Guard, Gate (Laravel terminology that does not match Casbin's model).

**Task** (pool):
A function submitted to a Pool for execution. Different from a queue **Job** (no envelope, no persistence, no retry policy) and from a cron **Job** (no schedule). Runs in a pool worker goroutine.
_Avoid_: Work, Unit.

**Environment** (Lua):
An isolated Lua VM (`*lua.LState`) that compiles and executes scripts. hex/lua provides the primitive; consumers attach whatever Go→Lua bindings they want.
_Avoid_: VM (implementation detail), Sandbox (implies stronger isolation guarantees than we make).

**Renderer** (TUI):
A component that converts hex markup into a target output format (ANSI terminal, plain text, Slack blocks, HTML). Different consumers pick different renderers; the markup is the shared IR.
_Avoid_: Formatter, Printer (both used elsewhere for different concepts).

**Queue**:
A named channel to which producers publish byte messages and from which consumers receive them, backed by a durable store (sqlite, redis, sqs) or in-process memory. Distinct from **Bus** (events): the bus is synchronous in-process pub/sub; a queue is asynchronous, durable, and cross-process.
_Avoid_: Channel (means Go primitive), Topic (used inside a queue to name partitions).

**Topic** (queue):
A named partition of a Queue that consumers subscribe to independently. Every published message belongs to a topic; a queue backend routes it to zero or more subscribers of that topic.
_Avoid_: Channel, Subject.

**Job**:
A typed unit of async work dispatched to a queue. Different from the cron **Job** (which is a scheduled trigger) — a queue job carries a payload, has retry semantics, and is executed by a worker consumer. When both concepts appear together in a codebase, prefer `queue.Job` and `cron.Job` explicitly.
_Avoid_: Task, Message (a message is the raw envelope; a Job is the structured payload inside it).

**Route** (web):
An HTTP endpoint registered with the echo server. hex/web owns the server, middleware stack, and lifecycle; consumers own the routes.
_Avoid_: Endpoint (used for connection targets), Handler (that's the func under the route).
