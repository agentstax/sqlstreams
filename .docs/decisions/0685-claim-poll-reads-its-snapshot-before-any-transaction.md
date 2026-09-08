---
status: superseded
date: 2026-09-06
phase: "pre-v1"
---

# A claim poll reads its snapshot before any transaction and opens the reclaim transaction only on an expired lease

## Context

Every cursor poll opened two transactions before it knew whether there
was anything to do: the reclaim transaction (BEGIN, UPDATE ... SKIP
LOCKED, ROLLBACK) ran unconditionally ahead of the fresh claim, and the
fresh claim opened its own transaction before taking the (head, xmax)
snapshot read. An idle poll was six round trips for one read-only
statement; a fresh claim was nine.

Measured on a local docker Postgres (RTT ~65µs, 41 partitions,
BatchLimit 100, `bench/claim`): the claim's SQL executes in ~150µs
server-side across all its statements; the rest of a 1120µs claim is
round trips, one WAL fsync, and payload transfer. The cursor CTE
statement itself runs in 29µs, so its shape is not where time goes.

## Decision

- `readClaimSnapshot` runs first, autocommit, one statement: the
  visible MAX(id), the snapshot's xmax, the cursor's proof columns, and
  `EXISTS (expired lease)` as `reclaimable`.
- The reclaim transaction opens only when `reclaimable` is true. A
  reclaim that finds nothing (a peer won under SKIP LOCKED) falls
  through to the fresh claim with the same snapshot.
- The caught-up short-circuit follows the reclaim branch, outside any
  transaction. Only then does the fresh claim BEGIN, straight into the
  cursor statement, taking the snapshot values as parameters.
- The gate CTE lists its three candidate (head, xmax) pairs as a VALUES
  table under one fence predicate, replacing two CASE arms.

## Consequences

- Idle poll: one round trip, no transaction (was six). Fresh claim: six
  round trips (was nine). Reclaim path: seven (was six; it pays the
  snapshot read it does not use).
- The fence stays sound: the pair is still read in an earlier statement
  than the cursor statement, and a read-only autocommit statement can
  never assign a txid, so the poll's own transaction can never sit below
  xmax. Under a REPEATABLE READ default the two statements no longer
  share one snapshot, which the old shape would have.
- Reclaim detection lags by the snapshot-to-reclaim gap, microseconds;
  granularity was already once per poll. An expired lease held locked
  forever keeps `reclaimable` true and runs the reclaim every poll, the
  old behavior.
- A missing cursor row now errors before the reclaim branch; before, an
  expired lease could still be handed out without one. [0387]'s loud
  error is the intended behavior.
- Supersedes the "idle poll short-circuits after the snapshot statement"
  mechanism of [0396] in shape, not in intent: the short-circuit still
  runs after the read-only snapshot, now with no transaction around it.
- **Rejected:** bounding MAX(id) by settled_head for partition pruning
  (the InitPlan becomes a correlated SubPlan, measured slower; the probe
  already stops at the first partition holding a row) and reading the
  sequence's last_value as head (the sequence read happens after the
  statement's snapshot, so an id issued in between by a transaction with
  xid >= xmax escapes the fence). Collapsing the fresh claim to one
  pipelined batch is parked in ROADMAP with its prototype.

Superseded by [0714](0714-consumer-observations-allocate-transaction-ids.md):
active observations must allocate their own transaction id; snapshot xmax
does not bound all already-running producers.
