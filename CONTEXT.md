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
