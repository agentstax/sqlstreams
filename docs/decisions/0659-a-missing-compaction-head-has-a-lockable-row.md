---
status: superseded
date: 2026-09-05
phase: "pre-v1"
---

# A missing compaction head has a lockable row

Superseded in part by [0660].

Amends [0530].

**Context.** The transactional compaction-head read locks the existing
`compaction_head_<topic_id>` row `FOR UPDATE`. No row exists for a new message
key, so two transactions can both read absence, calculate from the same empty
state, and commit competing first values. The head upsert serializes which
message wins; it cannot recover the lost read-modify-write input.

**Decision.** The `compaction_head` row becomes the one durable, lockable
identity for a message key participating in compaction. `head_id`,
`schema_version`, and `compaction_rank` are nullable with an all-null or
all-present check. `created_at` is immutable; `updated_at` changes when the
head changes or a headless row is locked again.

The transactional read loops: select the key row `FOR UPDATE`; when absent,
insert a headless row `ON CONFLICT DO NOTHING` and loop after a lost insert
race. A found headless row refreshes `updated_at` under its lock. Ordinary
compacted produces keep upserting this same row and may advance a null head,
so every writer coordinates through one PostgreSQL row-lock mechanism.

`TopicConfig` declares a positive defaulted TTL for headless rows. The topic
janitor deletes bounded batches where `head_id IS NULL` and `updated_at` is
older than the cutoff through a partial index and `FOR UPDATE SKIP LOCKED`;
active keys are neither waited on nor deleted. Cleanup is optional: a deleted
row is safely recreated by the transactional-read loop. Topic
observability reports the headless-row count and oldest age.

The public operation is
`client.Topic[Message](topicName).Key(messageKey).LockCompactionHead(ctx, tx)`.
The key handle owns both head reads: `CompactionHead(ctx)` is the plain read;
`LockCompactionHead(ctx, tx)` ensures and locks the key row, returning a nil
message when the row has no head. The producer instance loses
`GetCompactionHeadInTx`; it continues to own `ProduceInTx`. The locked path
resolves the topic name through the supplied transaction before touching its
generated table, so one operation never splits across database transactions.

**Consequences.** Every compaction-head query must handle the all-null state;
ordinary head reads and lists continue returning only materialized heads.
Pre-v1, the system and topic v1 baselines change in place; no migration step is
created, and an existing database must be recreated. The build needs a live
race proving two first updates compose, plus janitor-versus-reader cases where
each side obtains the row lock first. Scenario 05 names its topic and key
handles once, locks through the key, then produces through its registered
producer in the same transaction.

Rejected: advisory locks, which add a second protocol every compacted writer
must follow; serializable isolation, which changes the caller's whole
transaction; conditional writes, which make retry part of the public update
contract; and permanent headless rows with no bounded lifecycle.
