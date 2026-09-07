---
status: superseded
date: 2026-09-06
phase: "pre-v1"
---

# Collector progress contract and bounded history

## Context

Superseded by [0686](0686-history-based-pending-belongs-to-the-shared-alert-framework.md),
which expands adoption to existing alerts and reopens shared timing policy.

[0682](0682-metrics-export-uses-an-otel-producer-and-source-read-health.md) and
[0683](0683-alert-pending-duration-is-derived-from-measurement-history.md)
settled the direction. The user approved the concrete contract on the
Proposed metrics-export and alert-history concept pages before implementation.

## Decision

- Add only the system-scoped warning alert `metrics_collector_progress`;
  preserve the three existing alerts' conditions and schedules.
- Check every minute. MaximumAge defaults to max(2m, 3 collector poll
  intervals); PendingDuration defaults to 2m. These are the two configurable
  fields of MetricsCollectorProgressAlertConfig. Freshness and maximum gap
  are 2m. Fresh healthy evidence resolves; pending/insufficient evidence
  cannot resolve. Apply one resolved policy to the run's entire history.
- Record attribute-free gauges with unit s:
  `vulkan.metrics.collector.completed_timestamp` after the full pass succeeds,
  and `vulkan.metrics.collector.observed_completion_timestamp` on independent
  reads. Values are positive Unix seconds; the latter's 0 means no retained
  completion evidence. Read errors produce no observation.
- Use the Postgres clock and At.UnixMicro() compaction rank for these two
  series. Rank preserves observation-time head ordering despite late writes;
  other metrics retain their existing ordering. Actual execution reads,
  not scheduled times, supply evidence; retries of one sample reuse its key.
- Read the inclusive window [E - P - 2G, E], ordered by observation time
  then message id, without a row cap. Equal-time samples use the highest id
  in evaluation. Keep public History(ctx, limit) unchanged. Retention must
  exceed the window or be disabled; errors cannot become healthy evidence.
- Persist observations before evaluating alerts. Lock the existing alert
  compaction head, read current history after the lock, classify, and produce
  any transition in that transaction. Reuse LockHead/ProduceInTx; no new
  pending table. Reminders retain the existing 4h default and retention bound.
  Database evaluation time governs this check's reminder comparison.
- Export collection-local gauges `vulkan.otel.source.read_success` (no unit)
  and `vulkan.otel.measurements.rejected` ({measurement}). Count omitted rows
  once each, report `measurements cannot be exported` diagnostics, and return
  no error for partial rejection so healthy families survive periodic export.

## Consequences

The rank column can bound these built-ins' history without interpreting
measurement payloads in SQL. Missing rows never prove elapsed duration.
Concurrent decisions serialize through existing produced-alert state;
notification consumption remains retryable. Settings can change the next
verdict over existing evidence. Implementation proceeds in TODO chunks;
approval does not mark the proposed runtime behavior shipped.
