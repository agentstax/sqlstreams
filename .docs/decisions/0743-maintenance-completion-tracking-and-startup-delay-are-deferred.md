---
status: accepted
date: 2026-09-10
phase: "pre-v1"
---

# Maintenance completion tracking and startup delay are deferred

## Context

Review of [0742](0742-maintenance-settings-and-operations-are-separate.md)
identified successful-pass tracking and randomized initial vacuum delay as
extra features. The user requested their removal and separate Later items
for consideration.

## Decision

Supersede [0742](0742-maintenance-settings-and-operations-are-separate.md)
only in completion tracking and initial scheduling; retain its maintenance
configuration and operational model.

Remove RecordSuccess and InitialDelay from the tick runner. The first tick
is immediate. Success resets the failure streak only after recovery, using
the existing update without appending completion history. Remove the added
completion column, index, snapshot field, query, and feature-specific test.
Keep the existing jitter between subsequent passes and failure backoff.

## Consequences

Status reports operational targets, live claims, and failures, without a
last-success timestamp. This change requires no completion-history schema
addition. Both features remain separate Later considerations whose benefit
and cost must be evaluated before implementation.
