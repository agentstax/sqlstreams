---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Consumer defaults use small batches and responsive polling

## Context

BatchLimit 1 amortized no claim transactions; QueueSize still prefetched
one message. Five-second polling delayed local feedback and ready retries.
Larger batches need waiting allowance, and slow handlers have existing
queue, timeout, concurrency, and margin controls.

## Decision

- Set BatchLimit to 4, ClaimPollRate to 500ms, and QueueMargin to 15s.
  QueueSize continues to follow BatchLimit; MessageConcurrency remains 1.
- Keep the 30s message timeout, its following ceiling, 100ms TimeoutGrace,
  2s RecordMargin, and all other defaults. ShutdownTimeout remains derived
  from the ceiling plus grace and recording, not queue depth.
- Explain long-handler and quiet-fleet overrides in the consumer reference.
  Log resolved session budgets at startup; emit VK0105 once per locally
  tracked stale range, subject to existing instance warning suppression.
- Preserve the research and measurements in bench/consumerdefaults/RESULTS.md.
  The local PostgreSQL 17.10 suite measured batch 4 at 3.7 times batch 1's
  no-op throughput, quiet-arrival p95 of 484ms with 500ms polling, and
  950ms p95 in the repeated 50-group test. These are synthetic comparisons,
  not a production capacity guarantee. External systems support workload-
  specific prefetch and lease sizing, not one universal batch size.

## Consequences

Idle claim/retry traffic rises from about 0.6 to 6 queries per second per
consumer, excluding manager upkeep. The default range lease grows from
37.1s to 47.1s; crash recovery waits for expiry plus a later claim poll.
The derived shutdown budget stays 32.1s. Batch 4 increases reservation and
whole-range replay exposure; handlers still need idempotent side effects.

Long handlers should reduce both batch and queue, use realistic timeouts,
and size QueueMargin for preceding dispatches. Raising the handler timeout
and its ceiling together adds no queue allowance. Ordered same-key work
remains serial. Large quiet fleets may explicitly choose a 1s poll.

New sessions adopt zero-valued defaults without a database migration.
Explicit values remain unchanged, but an explicit QueueSize below 4 now
requires an explicit compatible BatchLimit. Live sessions are unchanged.
