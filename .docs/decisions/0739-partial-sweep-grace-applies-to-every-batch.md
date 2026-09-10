---
status: accepted
date: 2026-09-09
phase: "pre-v1"
---

# Partial sweep grace applies to every batch

## Context

[0738](0738-partial-message-sweeps-have-a-bounded-grace-period.md) relaxed
the eligibility cutoff after one batch, draining the entire TTL-expired
prefix. The user chose to remove that special case. The grace benchmark
executed no partial deletes, so it provides no evidence for that reset.

## Decision

Supersede [0738](0738-partial-message-sweeps-have-a-bounded-grace-period.md)
only in batch eligibility: every batch checks whether its oldest row is
older than TTL plus grace. Stop when the next batch remains within grace.
Keep the normal TTL predicate inside each eligible batch; a batch may
therefore include expired rows still within grace. Grace is a batch gate,
not a per-row minimum lifetime. Defaults, whole drops, cursor protection,
and eventual sparse-partition cleanup remain unchanged.

## Consequences

No cutoff reset or special first-batch behavior. Later batches can wait
until their oldest row exceeds grace. Cleanup scheduling and consumer
progress still affect actual removal time. Integration regression covers
stopping at the deferred next batch and later removing overdue rows.
