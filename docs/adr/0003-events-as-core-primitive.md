# Events are a core primitive, not opt-in

Every `*hex.App` creates a `*events.Bus` unconditionally, and the `hex.Application` interface includes `On()` and `Emit()`. The cost of an empty bus is near-zero (one allocated map). The CLI doesn't use events today, but having them available means providers and plugins can adopt event-driven patterns without framework changes. We considered making the bus opt-in via an option, but that would fork the `Application` interface into two flavors — one with events, one without — which defeats the purpose of a standard provider contract.
