# Cursor claim: where a claim's time goes

The hand-copied snapshot-xmax queries and one-batch prototype below are
historical. A deterministic concurrent-producer test exposed an unsafe
transaction bound; [0714](../../.docs/decisions/0714-consumer-observations-allocate-transaction-ids.md)
replaces it in the library. Only the real datastore calls use that fix.
These old microbenchmarks do not establish current throughput or delivery
correctness; use the reliability scenario for both.


`go run ./claim` from `.bench/`, against the running dev database
(`just database-*`). Registers its own topic, seeds 20,000 rows at
PartitionSize 500 (41 partitions), then times each statement of the
fresh claim on its own, the real `ClaimMessagesWithCursor`, and a
prototype of the one-batch shape parked in ROADMAP. The topic is
destroyed on exit.

## 2026-09-06, docker Postgres 17.10 on localhost, BatchLimit 100

Before [0685]; the real-path rows are the shape that entry replaced.

| step | p50 wall | server execution |
| --- | --- | --- |
| SELECT 1 (round-trip floor) | 63µs | |
| BEGIN + ROLLBACK | 128µs | |
| snapshot statement, MAX(id) over every partition | 88µs | 20µs |
| snapshot with `id > settled_head` (pruned, rejected) | 96µs | 59µs |
| reclaim UPDATE, no expired lease | 75µs | 23µs |
| cursor statement (old_values / gate / updated) | 78µs | 29µs |
| lease INSERT | 74µs | |
| readMessages, 100 rows | 216µs | 34µs |
| real ClaimMessagesWithCursor, backlog | 1120µs | |
| real Commit, no outcomes | 574µs | |
| real idle poll | 449µs | |
| prototype (one batch) claim, backlog | 788µs | |
| prototype idle poll | 102µs | |

One group draining the backlog, claims/s:

| instances | real | prototype |
| --- | --- | --- |
| 1 | 673 | 1020 |
| 4 | 1006 | 1098 |
| 8 | 981 | 1345 |

Readings:

- The SQL is not the cost. All five statements execute in ~150µs; the
  rest is round trips (9 per claim, 6 per idle poll at the time), one
  WAL fsync at commit (~350µs here), and payload transfer.
- MAX(id) is an ordered Append with LIMIT 1 touching 2 buffers; the
  per-partition cost is planning only, amortized by the prepared
  statement cache. Bounding it by settled_head made it slower.
- A zero-row reclaim UPDATE assigns no txid (checked with
  `pg_current_xact_id_if_assigned`), so a read-only snapshot before it
  stays sound.
- Beyond 4 instances the real path stops scaling on one group; the
  cursor-row lock is held across three client round trips per claim.
