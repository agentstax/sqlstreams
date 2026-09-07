---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Topic alerts evaluate every minute

## Context

History-derived pending is implemented, but hourly evaluation can postpone
an alert long after its two-minute pending requirement is satisfied. The
one-minute adoption target remained conditional on a cost checkpoint.

## Decision

- Default partition-count, compaction-read-cost, and worker-liveness schedules
  to @every 1m. Explicit expressions remain unchanged. Collector progress
  already checks every minute.
- Keep collector sampling at 30 seconds, pending and maximum gap at two
  minutes, retained metrics at 24 hours, and unchanged-alert repeats at four
  hours. No SQL, index, or recording redesign accompanies the cadence change.
- Accept the measured cost in bench/alertcadence/RESULTS.md: on disk-backed
  Postgres 18.4, the fuller 100-topic, 4.896-million-row case completed all
  three sequential evaluation/record passes in a median 14.459 seconds.
  This is evidence at the measured size, not a capacity guarantee.

## Consequences

Evaluation and alert-head updates run sixty times as often as hourly. The
three checks update 3N heads per combined pass even without alert changes;
larger installations retain the explicit cadence override. Scheduler delivery
and summary production add fixed costs outside the targeted timings.

Existing installations adopt defaults when System.Register redeclares their
schedules. Suspension remains preserved; Scheduler.Get exposes Expression.
Faster checking does not add collector evidence writes or change repeat policy.
