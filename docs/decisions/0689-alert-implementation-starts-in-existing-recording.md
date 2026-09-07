---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Alert implementation starts in existing recording

## Context

The user rolled back the standalone calculation from
[0688](0688-shared-alert-history-evaluation-contract.md) and requested an
implementation grounded in the existing alert system. All three workers
already converge on AlertController.Record and its classify function.

## Decision

- Supersede [0688](0688-shared-alert-history-evaluation-contract.md)'s
  standalone-calculator-first implementation. Keep history-derived pending,
  raw evidence, fresh recovery, and no user-managed state from [0686].
- First make Record's existing head read/classification/write atomic through
  LockHead and ProduceInTx. Return errors through existing worker handling;
  log transitions only after confirmed commit. Keep condition reads, clock,
  thresholds, schedules, registration warnings, and message content unchanged.
- Next deliver partition-count history through that recording path. Separate
  source reading and condition comparison only for real callers; reuse the
  existing condition for startup warnings. Add no detached evaluator hierarchy
  or speculative vocabulary before a runtime caller needs it.
- The Proposed page retains behavior goals; its names and intermediate shapes
  are not code mandates. Diagnose pending with the same calculation that
  records alerts. Revisit any required public-shape change on that page first.
- Resolve compaction applicability/pair identity and worker message details
  before adapting those conditions. Add collector progress after the existing
  checks use shared recording; OTel work remains afterward.

## Consequences

Quiet checks now acquire the existing head identity, creating an empty row
when needed. Competing decisions for the same key wait for commit. This does
not order condition reads taken before Record; the history integration must
refresh evidence after locking. It adds no table, timer, runner, or new
supported API. Targeted alert labs verify the actual worker/recording path.
