---
status: accepted
date: 2026-09-09
phase: "pre-v1"
---

# Idempotency expiry uses timestamp order

## Context

The user approved evaluating the remaining janitor sweeps after [0734].
Idempotency already has a created_at index, but its unordered limited
expiry query scanned a million young rows before statistics refreshed.
Empty compaction heads already have an appropriate partial expiry index.
Message-key leases lack an expiry index, but maintaining one adds writes.

## Decision

Order the idempotency expiry subquery by created_at before its batch limit.
Reuse the existing index; retain the timestamp predicate and UUID identity.
Add no boundary probe and no schema change. Leave empty-head cleanup alone.
Defer the lease expiry index: a scratch off/on/off comparison showed about
35% more insert WAL and 26% more renewal WAL with the index. Its empty-pass
benefit does not establish a net benefit for an active keyed workload.

## Consequences

Oldest expired keys are considered first, independent of caller UUID order.
In the measured young fixture, ordering changed a 32ms scan to a 0.08ms
index lookup before manual ANALYZE. This does not force an index plan in
all distributions or prove sustainable deletion capacity under load.
Existing registered streams already have the timestamp index; no migration.
A future lease-index decision needs keyed-workload evidence, including its
write cost. Pre-v1 adoption would edit baseline DDL and use fresh databases.

Evidence: scratch_sweeps_223540, scratch_leasecost_224005 and
scratch_janitor_224058 under .bench/scratchnative/results/evidence/native18/.
