---
status: superseded
date: 2026-09-10
phase: "pre-v1"
---

# Scheduled key vacuum is opt-in

## Context

Smaller key-delete batches did not reduce WAL per deleted row in the native
benchmark. Explicit VACUUM (ANALYZE) reduced retained key-table allocation
at similar observed throughput. The user approved optional scheduled
vacuum without changing the produce or consume paths.

Superseded in configuration ownership by [0741](0741-vacuum-settings-follow-stream-declarations.md).

## Decision

Declare a stream_vacuum worker with target_instances zero. Enable it with
target one. Use the existing manager, claims, heartbeat, retry, and failure
history. Run independently of janitor cleanup, with one request at a time.
Worker metadata defaults to a two-minute poll_rate and one-minute
vacuum_timeout. Randomize the initial delay and jitter subsequent delays;
wait after completion rather than accumulate scheduled requests.

The datastore executes ordinary VACUUM (ANALYZE) on the stream's key table
through its pool, outside a transaction. Keep autovacuum enabled. Preserve
explicit zero targets in worker declarations; sparse WorkerConfig still
defaults to one. Zero is valid InstanceTarget vocabulary for suspension.

## Consequences

Each running request occupies a client-pool connection and adds maintenance
I/O. Successful requests log duration; failures use worker diagnostics.
Metadata is read when claiming, so configuration changes require a new
instance. Re-registration restores metadata defaults and preserves the
operator's target. Existing streams need re-registration to declare the row.
Ten-minute benchmark observations do not establish an indefinite storage
plateau or a universal vacuum interval. No production throughput claim.
