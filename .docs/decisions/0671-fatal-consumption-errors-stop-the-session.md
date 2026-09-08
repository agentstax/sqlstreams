---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Fatal consumption errors stop the session

## Context

The exclusive lab surfaced a worker execution's non-lease error returning
through its manager. The manager cancels sibling executions and the error
reaches Consume. Suspending only that execution would leave a session running
with part of consumption permanently unavailable, such as exception processing.

Temporal's blocking Worker.Run returns fatal execution errors. Kafka Connect's
partial task failures have explicit status, failure details, and restart APIs.
RabbitMQ reports unexpected cancellation to the application. These contracts
support isolation at a caller-visible lifecycle boundary with observable failure.

## Decision

- Retain the existing fatal-error propagation through the consumer's manager
  to Consume. The consumer session is the caller-visible failure boundary.
- Keep requested shutdown and worker claim loss on their existing paths;
  claim loss permits replacement. Handler errors retain delivery policy.
- Return the fatal error to the caller, which owns reporting and restart policy.
  Do not add automatic suspension of one internal consumption execution.
- Do not change shared target_instances because one local execution stopped.
  Other sessions and replicas are not directly canceled by this session's exit.
- Document the cost: the session's sibling executions and paired system manager
  stop with it. Independent upkeep availability remains a lifecycle concern;
  this decision adds no separate process requirement or new upkeep mechanism.

## Consequences

No runtime behavior changes. Applications can isolate independent Consume calls
or deliberately cancel them together. A schema-incompatible instance reports its
failure without suspending compatible replicas through shared configuration.
Partial suspension would require a separately reviewed public failure-status
and recovery contract before it could replace this behavior.

Sources: [Temporal Go API](https://pkg.go.dev/go.temporal.io/sdk/worker),
[Kafka Connect](https://kafka.apache.org/42/kafka-connect/user-guide/),
[RabbitMQ cancellation](https://www.rabbitmq.com/docs/consumer-cancel).
