# hex/queue is layered: generic Queue at the core, Jobs on top

`hex/queue` defines a generic message-queue interface — `Publish(ctx, topic, []byte)` + `Subscribe(ctx, topic, handler)` — implemented by backend subpackages (`hex/queue/memory`, `hex/queue/sqlite`, later `hex/queue/sqs`, `hex/queue/rabbitmq`, etc.).

`hex/queue/jobs` sits on top and gives consumers a Laravel/Sidekiq-style job layer: named jobs with typed payloads, framework-owned retry with exponential backoff, dead-letter routing, and delayed dispatch. Jobs serialize to the underlying Queue as JSON envelopes.

Layering both means:

- Consumers who need raw pub/sub against SQS/Kafka use the core Queue directly.
- Consumers who want "dispatch a job, retry three times, move to DLQ" get the Jobs API without reinventing envelope + retry semantics per project.
- Backend authors implement one small interface (Queue) and get the job layer for free.

We rejected shipping Jobs-only because backends like SQS/SNS and Kafka are worth exposing raw for consumers that already speak those protocols. We rejected Queue-only because "wire up retry + backoff + DLQ" is exactly the boilerplate hex exists to eliminate.

## Not a queue backend: Temporal

Temporal is a workflow engine — durable stateful workflows with deterministic replay, child workflows, signals, queries, and timers. Its SDK constrains how you write code (workflow functions must be deterministic; activity functions run outside the workflow). Modelling it as a queue misrepresents both sides. If hex needs Temporal, it lands as `hex/workflow` — a separate abstraction wrapping the Temporal Go SDK, similar to how `hex/web` wraps echo.
