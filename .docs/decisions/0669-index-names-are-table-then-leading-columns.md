---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# An index is named for its table then its leading columns

## Context

The table-name and column review found the fourteen indexes split
between two naming schemes. Six were named for the columns they cover
(`_created_at`, `_message_key`, `_attempt`) and eight for the purpose
they serve (`_due`, `_expiry`, `_group`, `_topic`, `_worker`, the three
partial unique `_<owner>_name` ones). Purpose names read well in the
DDL and nowhere else: an operator meeting `worker_instance_expiry` in
`pg_stat_user_indexes` or an EXPLAIN plan has to open the DDL to learn
what it covers.

## Decision

An index is named `<table>_<columns>`: its table, then its leading
columns in index order, as many as it takes to be distinct from the
table's primary key and its other indexes. A partial predicate adds
nothing to the name; the WHERE clause is the DDL's job.

- A per-topic index carries its table's `%[2]s` verb as the prefix, so
  the name lands as `exception_queue_<id>_consumer_group_id_message_key`.
- The three partial unique indexes on worker_config all lead with
  `name`, so they take two columns: `worker_config_name_topic_id`,
  `worker_config_name_consumer_group_id`, `worker_config_name_system_id`.
- `tools/conventions` walks every CREATE INDEX literal under pkg/ and
  fails when the suffix after the table prefix is not a leading run of
  the index's column list.

Postgres names are 63 bytes; the longest name here is 56 with a
ten-digit topic id, so the scheme has room but a new index on a
per-topic table checks its length.

## Consequences

Twelve index names change in the baseline DDL and the sandbox mirrors.
Nothing else names an index: no query, Row struct, lab, or doc page.
The Go variables and mirror file names that hold the statements keep
their purpose names (`createScheduleCursorDueIndexSql`); they name the
statement, not the index, and are not operator-facing.
