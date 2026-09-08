---
status: accepted
date: 2026-09-05
phase: pre-v1
---

# 0653 — Consumer names the public group handle

Supersedes the affected naming clauses of [0644] and [0646].

## Context

[0646] nested the consuming handle under its typed topic but changed its name
from Consumer to Group. That nesting requires a generic handle; it does not
require the Group name. Go permits a `Consumer` method beside a `Consumer`
type, and `pkg/consume.Consumer` does not collide with
`pkg/consumer.Consumer` because the declarations live in separate packages.

Consumer group remains the domain noun for the durable row, SQL tables,
diagnostics, metrics scopes, and operator commands. On the Go facade, however,
the caller selects the consumer that will register and run, so Group makes the
call tree switch nouns at its most common step.

## Decision

The typed topic handle names a consumer with
`Topic[Message](name).Consumer(groupName) *ConsumerHandle[Message]` and lists
the materialized values with `Consumers(ctx) ([]*Consumer, error)`. The
consumer's nested observability handles are `ConsumerMetricsHandle` and
`ConsumerAlertsHandle`.

The materialized value is declared as `consume.Consumer` so vulkan's alias
keeps the declaration's name. Controller verbs, datastore rows, table and
column names, errors, owner kinds, metrics types, log attributes, CLI commands,
and prose continue to say consumer group where they describe that mechanism.

## Consequences

The Go rename is compile-time breaking and JSON is unchanged. There is no
compatibility alias: pre-v1 callers update `Group` to `Consumer` and `Groups`
to `Consumers`. The topic still owns the message type, and every verb continues
to resolve the same consumer-group rows through the same controller path.
