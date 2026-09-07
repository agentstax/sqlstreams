---
status: superseded
date: 2026-09-07
phase: "pre-v1"
---

# Partition alerts calculate retained duration

Superseded by [0699](0699-alert-timing-fields-are-inline.md) for configuration
shape; the retained-history behavior below remains in effect.

## Context

The shared evaluation result [0694] separates healthy from pending and missing
evidence. The next implementation chunk connects partition count to the retained
history supplied by metrics [0693], using the storage-time contract [0690].

## Decision

- Every Evaluator accepts the consumed JobPayload. PartitionCountAlertConfig
  adds Pending: Duration and MaximumGap default to two minutes; Disabled permits
  immediate activation with fresh evidence. Older payloads resolve these defaults.
- Partition-count Evaluate reads one database-time window through metrics.
  GetMeasurementHistory returns that evaluation time and the existing compaction
  CreatedAt-window read, ordered by CreatedAt/id descending without a row cap.
  Retention must exceed the requested window or be disabled.
- Read Duration + 2*MaximumGap, or MaximumGap when disabled. Reject nonpositive
  durations and overflowing window arithmetic before reading. Highest id wins
  timestamp ties. Healthy/unusable samples and excessive gaps break the span;
  activation requires the newest-to-earliest sample span to reach Duration.
- Fresh healthy evidence resolves through Record. Missing, stale, or unusable
  evidence returns insufficient_evidence, leaves the alert unchanged, and counts
  as a failed-topic check. Read/decode errors keep their existing error path.
- Registration warnings use the same Evaluate with pending disabled and require
  fresh evidence. They log insufficient evidence and never produce alerts.
- Other conditions keep live reads until metrics owns their required evidence.
  Alert cadence stays hourly; no measurement production moves into alert code.

## Consequences

An overnight gap starts a new unhealthy span. Waiting alone cannot turn one
sample into an active alert. Delayed writes count as fresh storage-time evidence.
The observed span is calculated again on each check; no timer, cursor, or pending
state table exists. Metrics' unused absolute-time wrapper is replaced by the
database-time history read; the underlying compaction method is unchanged.
Race/unit checks cover the calculation and payload policy. Database integration
and alert-lab checks remain deferred to the review checkpoint.
