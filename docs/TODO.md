# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

## A missing compaction head has a lockable row [0659]

Invariant: one `compaction_head_<topic_id>` row is the lockable identity for a
message key participating in compaction. Its three head fields are either all
null or all present. A transactional lock composes first writes as well as
updates; TTL cleanup may remove an inactive null-head row but is never required
for correctness.

Settled this session, do not reopen: nullable head fields plus `created_at` and
`updated_at`; a positive one-hour defaulted topic TTL; bounded janitor cleanup
with `FOR UPDATE SKIP LOCKED`; headless-row count and oldest age in topic
observability; `KeyHandle.LockCompactionHead(ctx, tx)` owns the public lock;
`ProducerInstance.GetCompactionHeadInTx` is deleted. Pre-v1 means both v1
baselines change in place and existing databases are recreated, not migrated.

Build order -- keep each chunk green with the listed foreground checks before
starting the next:

1. **The doc-site proposal.** DONE 2026-09-05. The reviewed
   Proposed contract is in the client guide:
   the `CompactionHead` / `LockCompactionHead` pair, nil for a locked row with
   no head, transaction-consistent topic resolution, `EmptyCompactionHeadTTL`
   and its one-hour default, and the full scenario 05 read-modify-write. Review
   that page before code; keep it labeled Proposed until chunk 8 is green.
   Check: targeted Prettier, Remark, Vale, and website build.
2. **Config and fresh-schema baseline.** DONE 2026-09-05. Added `EmptyCompactionHeadTTL` to
   `TopicConfig`, `Topic`, the controller row/adapters, every get/list/register/
   replace/config-log query, CLI config output, and both `topic_config` baseline
   tables with a one-hour database default. Change `compaction_head` baseline
   DDL: the three head columns become nullable as one unit, add the all-null or
   all-present check, and add `created_at` / `updated_at`. Add config-default,
   validation, and round-trip coverage. No system/topic migration registry
   entries. Checks: build and `go test -race` on topic, system, admin, and CLI
   packages; schema and registration labs against a fresh database.
3. **One internal ensure-and-lock path.** DONE 2026-09-05. Moved the
   transactional operation into the compaction domain. One `INSERT ... ON
   CONFLICT DO UPDATE ... RETURNING` creates or locks the key row and refreshes
   `updated_at` only when it has no head [0660]. Pointer fields stay on the
   table-exact row that can carry nulls. Focused validation and adapter tests
   cover the boundary. Checks: build and `go test -race` on compaction,
   produce, topic, and admin packages.
4. **Every existing head consumer handles null.** DONE 2026-09-05. Changed the
   produce upsert to advance `head_id IS NULL` and set `updated_at` whenever the
   winner changes.
   Audit every compaction-head query: ordinary head/list reads keep inner-join
   materialized-only behavior; `IsCompacted` ignores null heads; rank,
   retention, key leases, schedule status, schema health, and metrics neither
   scan null into a scalar nor classify a null row as a head. Checks:
   compaction, compaction-rank, compaction-head-race, compaction-head-retention,
   key-lease, and schema-evolution labs.
5. **Public handle move.** DONE 2026-09-05. Added
   `Topic[Message](name).Key(messageKey).LockCompactionHead(ctx, tx)` and routed
   it through admin -> topic controller -> compaction controller. Topic
   resolution and schema gating use the supplied transaction, never the pool.
   Deleted `GetCompactionHeadInTx` at every producer layer and facade; updated
   the facade tests. Checks passed: build, `go test -race ./pkg/vulkan ./pkg/admin
   ./pkg/compaction/... ./pkg/produce/...`, and examples module build.
6. **Topic-janitor TTL.** DONE 2026-09-05. Added one
   `SweepExpiredEmptyCompactionHeads` controller and datastore path using the
   topic's TTL and existing sweep batch size. It selects expired `head_id IS
   NULL` rows in `updated_at` order with `FOR UPDATE SKIP LOCKED`, deletes that
   bounded set, and logs only a nonzero count at Debug. Added a partial
   `(updated_at, compaction_key)` index for null heads and called the path from
   the existing topic-janitor sweep, with no new worker or runtime loop. Checks
   passed: topic race tests plus retention and sweep labs.
7. **Point-in-time observability.** Extend `TopicSnapshot` and its one metrics
   query with `CompactionRowsWithoutHead` and
   `OldestCompactionRowWithoutHeadAge`; zero age means none. Keep the existing
   `Compacted` metric tied to a materialized head, and add no second query or
   built-in time series without a consumer. Checks: metrics controller race
   tests, metrics lab, and metrics-collector lab.
8. **The one live concurrency lab and scenario 05.** Add a focused lab that
   proves: two transactions first-increment one absent key to 2; an ordinary
   compacted produce fills a locked null-head row; committing without produce
   leaves a row the TTL later removes; a non-null head never expires; and both
   janitor-first and locker-first races converge without a missing lock or
   blocked sweep. Rewrite playground scenario 05 around named topic/key
   handles and `LockCompactionHead`; run the lab under `-race` and build every
   example.
9. **Review-ready closeout.** Remove Proposed from the client guide and update
   table design, config reference, playground scorecard, and [0659] only where
   implementation discovered a real consequence. Recreate the database, run
   the full fresh-DB lab suite once, `just verify`, and the website build. At a
   release checkpoint also run the prior-tag compatibility lab and update the
   migration compatibility table. Move the shipped summary to HISTORY, remove
   this TODO window and the [0659] ROADMAP item.
