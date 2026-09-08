---
status: superseded
date: 2026-09-07
phase: "pre-v1"
---

# Consumer observations allocate transaction ids before advancing cursors

## Context

Two 32k/s throughput probes left 131 and 57 committed messages without
handler calls while their group's cursor advanced past them. A two-message
regression reproduced the unsafe claim: transaction A gets an earlier xid,
B gets a later xid and inserts message 1, A inserts message 2 and commits,
then the consumer claims through 2 while B is still open.

PostgreSQL sets snapshot xmax to latestCompletedXid + 1, not next-unissued
xid. B can already own message 1 with xid >= snapshot xmax. See
[PostgreSQL's transaction implementation](https://github.com/postgres/postgres/blob/REL_18_STABLE/src/backend/access/transam/README)
and [snapshot semantics](https://www.postgresql.org/docs/18/functions-info.html#FUNCTIONS-PG-SNAPSHOT).
Separately, empty claims rolled back their pending observation, preventing
later polls from using it during continuous traffic.

## Decision

Supersedes the fence definition in [0394](0394-snapshot-fence-claims-stop-at-proven-head.md),
[0389](0389-fanout-marked-scan-lifecycle-cursor-rows.md), and
[0685](0685-claim-poll-reads-its-snapshot-before-any-transaction.md).
Their cursor, lease, fan-out, and reclaim structure remains.

An active observation reads the visible head and allocates pg_current_xact_id
in one autocommit statement. Its snapshot precedes that xid allocation;
every producer that could own an id <= head already has a smaller xid.
The following claim/scan statement proves a pair when its snapshot xmin
is >= the observation xid. The observation transaction has already ended.
The existing pending_xmax column stores this bound; fresh/stored pairs and
settled_head retain their roles. Empty claims commit their pending state.
Caught-up observations short-circuit xid allocation as well as cursor writes.

## Consequences

No producer serialization, timing allowance, new table, or schema change.
Active polls now consume a transaction id; idle polls remain read-only.
The producer must still acquire an xid before allocating message ids, and
message sequences must retain CACHE 1. Deterministic tests cover both claim
and fan-out loss, empty-claim persistence, and idle xid allocation.

Existing state is not repaired by this code change. With all consumers
stopped, discard old unproven observations before resuming: set settled_head
and pending_head to claimed, and pending_xmax to NULL on each cursor table.
Do not move claimed or committed backward blindly: already skipped messages
need separate reconciliation, and replay may repeat handler side effects.
Benchmark validation uses fresh databases. Archived claim microbenchmarks
using snapshot xmax cannot establish correctness or current throughput.

Superseded in scope by [0715](0715-throughput-excludes-archived-delivery-consumer.md):
keep the cursor fix; exclude the archived delivery consumer.
