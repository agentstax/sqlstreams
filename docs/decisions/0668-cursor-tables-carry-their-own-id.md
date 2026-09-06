---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# A cursor table carries its own id and the owner's id as UNIQUE

## Context

The two `_cursor` tables were keyed differently. consumer_group_cursor
had `id BIGSERIAL PRIMARY KEY` and `consumer_group_id ... UNIQUE`;
schedule_cursor used `schedule_id` as its primary key with no surrogate.
Nothing reads consumer_group_cursor's `id`: all fourteen literals key
by the owner column. The review proposed dropping the surrogate and
keying both by the owner, the compaction_head shape.

## Decision

The surrogate stays and schedule_cursor gains one: every `_cursor`
table carries `id BIGSERIAL PRIMARY KEY` as its first column and the
owner's id as `NOT NULL UNIQUE REFERENCES ... ON DELETE CASCADE`.
A sequence id that nothing reads today keeps the table open to
later changes -- a cursor that stops being 1:1 with its owner, a
row another table needs to reference -- without a key migration.

The rule is scoped to the `_cursor` kind. The tables keyed by a
natural or composite key -- compaction_head, idempotency_key,
exception_queue, claim_lease, message_key_lease -- keep their keys:
they are the hottest write paths and their upserts are built on
`ON CONFLICT` over those keys.

`tools/conventions` walks the baseline DDL and fails on a `_cursor`
table whose first column is not `id BIGSERIAL PRIMARY KEY`.

## Consequences

schedule_cursor grows an id column and a sequence; its one INSERT
names its columns, so no literal, Row struct, lab, or doc page
changes. The 1:1 guarantee moves from the primary key to the UNIQUE
constraint, which is the same index the lookups already use.
