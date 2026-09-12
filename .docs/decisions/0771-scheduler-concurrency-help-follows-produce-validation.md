---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# Scheduler concurrency help follows produce validation

## Context

The shared concurrency enum accepts ordered, but Scheduler.Run always enables
message compaction. Produce validation rejects ordered with compaction because
compaction supersedes older messages while ordered delivery preserves their order.
A disposable-Postgres run confirmed that the enum alone is not the accepted set.

## Decision

- Supersede [0770]'s concurrency-help choice: scheduler run advertises parallel
  and exclusive, and explains why ordered cannot run a compacted schedule.
- Keep [0770]'s --builtin flag, system binding list, direct resource JSON,
  null/exit-1 absence, and declared alert examples.
- Keep the existing client and produce validation; this changes help only.

## Consequences

The CLI describes policies that complete through the client's full run path.
Passing ordered still returns the existing compaction-conflict error and
produces no message. Parallel remains the default.
