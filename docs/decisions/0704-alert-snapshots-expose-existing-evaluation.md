---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Alert snapshots expose existing evaluation

## Context

Recorded alerts cannot explain pending or insufficient evidence. The user
approved a read-only Snapshot contract for all four built-ins after reviewing
its return fields, current-declaration policy, and failure behavior.

## Decision

- Add AlertHandle.Snapshot(ctx), resolved through admin and the existing
  schedule controller. Evaluate retained evidence under the current declared
  schedule payload, including resolved defaults. Never consume a message.
- Use one evaluation value: extend and rename AlertEvaluationResult to
  AlertEvaluationSnapshot. Existing condition evaluators return the snapshot;
  scheduled workers still pass it to Record. No parallel calculator or store.
- Return State, Finding, EvaluatedAt, ObservedAt, UnhealthySince,
  ObservedDuration, PendingDuration, DisablePending, MaximumGap, MaximumAge,
  and a display-only insufficient-evidence Reason.
- Topic observations use stored time; collector progress uses completion time.
  Healthy/insufficient results have zero unhealthy span. Disabled topic pending
  skips span calculation; collector progress keeps its coverage-backed span.
- Unsupported names/scopes, missing schedules, malformed policies, and read/
  decode failures return errors. Missing/stale/unusable evidence returns an
  insufficient snapshot. Suspended schedules remain inspectable.

## Consequences

Snapshot explains the current declaration, not the last or next queued check.
Queued checks retain consumed policy. Latest/History retain their recorded
active/resolved contracts and can differ from Snapshot. Snapshot performs no
measurement writes, alert writes, head locks, or cursor updates. Existing
recording semantics remain unchanged; no compatibility layer is added.
