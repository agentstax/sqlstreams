---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Alert pending duration is derived from measurement history

## Context

Users consume Vulkan's alerts topic and should not reconstruct startup grace
or pending timers. A separate alert cursor with timestamps and revisions
duplicates timing that retained measurement history can establish.
Collector completion history alone cannot distinguish downtime from a
collector that stopped progressing while other checks continued.

## Decision

- Alongside [0682](0682-metrics-export-uses-an-otel-producer-and-source-read-health.md),
  supersede [0678](0678-metrics-export-distinguishes-read-success-and-observation-freshness.md).
  Derive pending duration from consecutive qualifying observations in a
  sufficient retained history window. Fixed evidence, evaluation time, and
  policy produce the same result. Add no pending-state table or persisted
  pending deadline/revision mechanism.
- Gaps beyond the condition's allowance break the sequence. A single old
  observation cannot become sufficient evidence by waiting. Evaluator
  restarts do not reset recorded measurement history. Retention loss or
  insufficient coverage cannot establish a healthy result.
- Core owns evaluation and produces actionable active/resolved alerts using
  existing alert machinery. Fresh healthy evidence can resolve an active
  alert; pending or insufficient evidence cannot resolve it automatically.
  Repeats and concurrency remain production concerns, not implied guarantees
  of deterministic evaluation. Users maintain no pending state.
- Record collector completion only after the full pass and measurement
  writes succeed. Independently sample its progress through existing core
  scheduled-alert machinery and retain the observations in metric history.
  Evaluate historical samples against their own observation times. A read
  error records no fabricated observation and does not establish recovery.
- Collector progress replaces the generic per-series freshness companion
  proposal. Keep original observation times available through core's API;
  unchanged counters and custom cadence need condition-specific treatment.

## Consequences

After downtime, new observations must establish the pending window before a
new activation. The independent check adds recorded evidence, not a new
user-managed server. Vulkan cannot report an outage while all its checking
machinery is stopped. Observation representation, cadence, thresholds,
history ordering/coverage, and targeted checks remain implementation review
items on Proposed concepts/metrics-export; no library behavior ships here.
