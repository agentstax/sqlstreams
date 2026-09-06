---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Defaults use the normal client API

## Context

The documentation pass [0581] left a proposal for DefaultProducer and
DefaultConsumer, plus a public API comment audit. The one-client API [0625]
subsequently removed the constructor friction that motivated that proposal.
The quickstart now uses LifecycleContext and explains cancellation, but the
public Consume comment omitted that requirement.

## Decision

Retire the separate Default constructors proposal. Normal client and handle
constructors already accept nil configs and provide the common defaults;
there is no separate quickstart-only behavior to warn against in production.

Fold the remaining audit into one public API documentation review after
public surface trim. Comment sweeps are its execution list. Apply the current
conventions to retained handles, instances, and aliased declarations, covering
meaningful defaults, lifecycle requirements, errors, and destructive effects.

Correct the public roadmap and the identified Consume, Rename, Health, and
schedule Destroy comments now. Clarify the quickstart's explicit cancellation
opt-out and its effect on embedded upkeep. The full audit remains future work.

## Consequences

There is one construction path for quickstarts and production. Cancellation
requirements are visible at the public method, and the documentation describes
existing behavior. No runtime or SQL changes are required.
