---
status: superseded
date: 2026-09-06
phase: "pre-v1"
---

# Shared alert history evaluation contract

## Context

Superseded by [0689](0689-alert-implementation-starts-in-existing-recording.md)
after the user rolled back the standalone calculation implementation.

[0686](0686-history-based-pending-belongs-to-the-shared-alert-framework.md)
placed pending in the existing alert framework. The user approved the revised
step 3 proposal and requested the shared calculation. Compaction pairing and
worker-detail evidence remain explicit gates for those checks' adoption.

## Decision

- AlertPendingConfig owns Duration (2m), MaximumGap (2m), and Disabled (false).
  MaximumGap also bounds freshness. Zero fields default; immediate activation
  is explicit. Resolved durations must be positive. Validate history-window
  arithmetic; disabled pending needs only the freshness window.
- Put one pure EvaluateHistory calculation in pkg/alert/controller. Each
  condition maps retained raw evidence to transient AlertObservation values:
  stored message id, observation time, and healthy/unhealthy/unusable status.
  These interpretations are never persisted in place of raw history.
- Order by observation time then id, highest id for ties. Preserve caller
  inputs. Use [E-P-2G,E], or [E-G,E] when disabled, without a row cap. A
  healthy/unusable sample or excessive gap breaks the unhealthy sequence.
- Return AlertEvaluationSnapshot with status, evidence times, supported span,
  resolved timing, and missing/stale/unusable reason. Active spans are lower
  bounds within the window, not incident age; immediate mode reports zero span.
- Collector code only interprets completion timestamps and resolves its age
  threshold. Shared code owns duration/freshness. Retention must exceed the
  required window or be disabled. No timer/cursor state or added runner.
- The approved adoption target is one-minute checks, subject to targeted cost
  verification before changing existing hourly schedules. AlertHandle.Snapshot
  will reuse this calculation read-only; Latest/History remain recorded alerts.
  Current schedule policy governs checks; registration warnings stay log-only.
- Keep partition count as the first runtime integration checkpoint. Resolve
  compaction pair identity and worker detail retention before their adoption;
  this approval does not authorize a snapshot store or reduced alert messages.

## Consequences

The calculation is reusable without datastore access or a constructed runtime
controller. Only fresh healthy evidence can permit recovery. Runtime recording,
schedule changes, and the public Snapshot method remain unimplemented; the
site retains Proposed labels. OTel producer and reader work is unchanged.
