---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Collector progress uses manager lease history

## Context

Worker instance logging [0700] [0702] supplies independent manager coverage.
Collector progress must use the existing scheduled alert and recording paths
without requiring another observation series or user-maintained pending state.

## Decision

- Read the latest collector completion and manager snapshots intersecting the
  pending window, as of database time. Order snapshots by original creation
  time descending, then expiry and id descending; include boundary intervals.
- Recent completion is healthy. Otherwise, no current lease coverage is
  insufficient evidence. Join touching/overlapping intervals backward;
  uncovered gaps break pending, including overnight shutdowns.
- Unhealthy duration begins at the later of coverage start and completion
  plus MaximumAge, bounded by the read window. Absent completion uses coverage
  start. Share the pending-duration decision with sampled-history alerts.
- Register metrics_collector_progress (VK0101) as a system-owned scheduled
  alert, with the normal consumed policy and atomic Record/classify path.
  Insufficient evidence logs a warning and leaves recorded alerts unchanged.
- Add SystemConfig.MetricsCollectorProgressAlert and the system alert selector.
  Default cadence is one minute, pending two minutes, and MaximumAge zero
  resolves to max(two minutes, three declared collector poll intervals).
  A positive MaximumAge overrides this; DisablePending still requires coverage.
- Exact lease gaps need no MaximumGap config. Existing topic alert schedules
  remain hourly. Completion age uses the metric's Unix-second value.

## Consequences

No additional measurement writes or pending-state storage are needed. Current
declarations can change the default age before a running collector next claims
its updated rate. Lease coverage retains [0700]'s last-expiry tradeoff and
does not prove successful execution. No internal alert can report while all
checking machinery is stopped. Real-worker labs remain a separate checkpoint.
