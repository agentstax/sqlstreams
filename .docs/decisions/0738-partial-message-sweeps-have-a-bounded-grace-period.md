---
status: accepted
date: 2026-09-09
phase: "pre-v1"
---

# Partial message sweeps have a bounded grace period

## Context

Partial message deletion consumed193s of completed SQL in the paired
600s native experiment relaxed_pair_20260910_003620. Bypassing it reduced
expired-key backlog, but leaves sparse/current partitions without row cleanup.
The user approved implementing bounded deferral directly, without proposal docs.

## Decision

Add partial_sweep_grace_period to per-stream janitor worker metadata beside
poll_rate and sweep_batch_size. It is a nonnegative duration in nanoseconds;
missing or zero preserves current behavior. Include it in the startup log.
Use the existing indexed oldest-row lookup to gate the first batch of each
partition at retention TTL plus grace. Once eligible, drain the expired
prefix using the original TTL cutoff, including subsequent batches.
Whole-partition drops retain their existing TTL and consumer-progress guards.
Partial sweeps retain their existing consumer-progress and orphan cleanup.
Debug records identify deferred partitions, oldest-row age (duration), and
eligibility age (threshold); this change adds no metric collection queries.

## Consequences

Grace trades delayed row cleanup for opportunities to drop a whole partition.
Sparse/current partitions still become eligible; grace is not a hard physical
retention bound because scheduling, failures, and consumer progress can delay
cleanup further. The existing approximate ID/timestamp ordering remains.
No schema migration and no change to key cleanup or DROP lock handling.
The 30s example is not a benchmark-validated default; the default stays zero.
