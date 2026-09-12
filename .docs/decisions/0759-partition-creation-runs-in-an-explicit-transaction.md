---
status: accepted
date: 2026-09-11
phase: "pre-v1"
---

# Partition creation runs in an explicit transaction

## Context

`produce.ensureCoveringPartition` ran `SET LOCAL lock_timeout`, the
advisory lock, and the CREATE as one pgx batch outside a transaction, on
the premise that the batch's implicit transaction scoped the SET LOCAL
in one round trip. Postgres logged `WARNING: SET LOCAL can only be used
in transaction blocks` on every partition creation, and whether the cap
applied was unestablished. The janitor's drop and the migrate step
already open an explicit transaction for the same SET LOCAL.

## Decision

- Measured against Postgres 18 with a second session holding the lock:
  the batch form did apply the cap (55P03 after the timeout, no leak to
  the pooled connection) and emitted the warning on every call; the
  explicit-transaction form applied the cap with no warning.
- `ensureCoveringPartition` now opens `BeginTx`, runs the SET LOCAL, the
  advisory lock, and the CREATE as three statements, and commits; a lost
  IF NOT EXISTS race still returns nil through the deferred rollback.
  The site matches the janitor and migrate sites.
- Rejected: queueing BEGIN and COMMIT inside the batch. It silences the
  warning in one round trip, but a failed statement skips the queued
  COMMIT and leaves the pooled connection in an aborted transaction,
  which the pool then destroys on release.

## Consequences

Partition creation costs two more round trips, on a path that runs once
per partition per stream and in the background for create-ahead. The
server log no longer carries a warning per partition; an operator
grepping it finds only real conditions. The create-ahead attempt
allowance (two lock waits plus slack) is unchanged.
