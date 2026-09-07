---
status: superseded
date: 2026-09-06
phase: "pre-v1"
---

# History-based pending belongs to the shared alert framework

## Context

The unconnected classifier from [0684](0684-collector-progress-contract-and-bounded-history.md)
places duration policy under collector progress. Existing checks still emit
their first positive result through shared alert recording. The user approved
making pending a shared capability and proving it on an existing check first.

## Decision

Superseded by [0690](0690-alert-history-uses-stored-message-time.md), which
replaces the rank-read choice and preserves the shared alert framework.

- Supersede [0684](0684-collector-progress-contract-and-bounded-history.md)'s
  collector-only adoption scope. Keep its OTel export contract, collector
  completion/observation semantics, and bounded-history guarantees except
  where shared timing choices below require review.
- Each check owns its condition and raw-evidence interpretation. The existing
  alert controller owns one history-derived duration calculation and the
  active/resolved/repeat path. Retain healthy as well as unhealthy evidence;
  current policy can reevaluate raw history. No pending timer table or runner.
- Pending/insufficient evidence cannot resolve an active alert. Only fresh
  healthy evidence resolves; observation gaps cannot earn pending duration.
  Reuse the existing alert head and produce-transaction seam for decisions.
- Keep the completed rank-bounded history read. Reopen the collector-specific
  calculation, preserving useful tests. Implement partition count end to end,
  then compaction read cost and worker liveness, then collector progress.
- Worker checks retain independent snapshot reads and their topic-scoped
  condition. The metrics collector must not become their observation source.
- Review shared config, immediate mode, cadence, pending/gap/freshness
  defaults, per-owner evidence representation, and diagnostic visibility on
  the Proposed site page before code. The suggested one-minute/two-minute
  policy is not approved; existing checks currently default to hourly.
- OTel producer/reader work follows the shared alert implementation. Users
  still consume actionable alerts without maintaining startup or pending state.

## Consequences

The first implementation checkpoint exercises an existing alert rather than
adding another process. More frequent checks increase database work, so the
cadence review includes targeted cost verification. Pending and insufficient
evidence need inspectable diagnostics; the read surface remains to be agreed.
Runtime code stays unchanged during this proposal and task-plan revision.
