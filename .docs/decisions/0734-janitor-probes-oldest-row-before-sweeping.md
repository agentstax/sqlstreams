---
status: accepted
date: 2026-09-09
phase: "pre-v1"
---

# Janitor probes oldest row before sweeping

## Context

An expiry query can scan a million unexpired messages and delete nothing.
The user accepts approximate ordering of message ids and created_at,
including the existing whole-partition expiry approximation, and approved
an oldest-row precheck before pursuing partition lifecycle changes.

## Decision

Each row-sweep batch reads created_at from the lowest visible message id
through the partition primary key. An empty partition or a timestamp at
or after the sweep cutoff ends that partition's row sweep for this pass.
Repeat the check between batches so a drained expired prefix does not
lead to a full scan of its young remainder. Keep the existing per-row
expiry predicate, consumer committed floor, and transactional associated
row cleanup. Whole-partition drops and idempotency expiry are unchanged.

## Consequences

No producer metadata writes, schema changes, or new configuration.
Out-of-order timestamps can delay cleanup of an older higher-id row until
the first row expires; the precheck does not authorize extra deletion.
An unvacuumed deleted prefix can still make the index probe expensive.
The existing janitor timeout/error reporting remains; a no-change sweep
stays silent. Query-cost evidence is not a sustained-throughput claim.

Evidence: .bench/scratchnative/results/evidence/native18/scratch_probe_221720
compares the old scan, the probe, and the deleted-prefix/vacuum limitation.
