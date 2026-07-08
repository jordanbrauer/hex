# hex/telemetry wraps OpenTelemetry

`hex/telemetry` wraps OpenTelemetry for tracing and metrics. OTel is the CNCF standard, works with every modern backend (Grafana, Datadog, Honeycomb, Jaeger, Prometheus-scrapable stdout, etc.), and its Go SDK is the reference implementation. Rolling our own would trade portability for zero gain.

Following the pattern used for cron/log/web/lua/pool/policy/i18n/featureflag: hex owns one-call setup that wires tracer provider + meter provider + shutdown, exposes type aliases so consumers get the full OTel API through the wrapper, and ships two exporter defaults (stdout for dev, OTLP-gRPC for production). Consumers who want other exporters (jaeger, prometheus scrape, zipkin, azure monitor, xray) import them from OTel directly and pass through Options.

## v1 exporters

- **stdout** — dev/debugging default. Prints spans and metrics as JSON.
- **otlp-grpc** — production default. Ships to any OTLP-compatible collector (Grafana Alloy, Datadog Agent, OTel Collector, Honeycomb, etc.).

The two shipped exporters cover 95% of real-world setups. Others are one line of importing and passing.

## No logger bridge in v1

hex/log wraps charmbracelet/log which does not natively speak OTel logs. Bridging it would either force a rewrite of hex/log to use log/slog + otelslog, or write a custom writer that emits OTel LogRecord — both bigger than one phase. Deferred to a follow-up.

Traces still work with hex/log: consumers can log the trace ID / span ID from the current context by hand, or add a hex/log helper later.
