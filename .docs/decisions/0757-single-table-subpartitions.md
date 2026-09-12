---
status: accepted
date: 2026-09-11
phase: pre-v1
---

# 0757 — Retain per-stream tables instead of LIST/RANGE subpartitioning

## Context

We create and manage a set of tables `*_<stream_id>`. Direct SQL queries require
users to resolve the stream id and put it into the table name.

This review found no historical record explicitly rejecting LIST/RANGE
subpartitioning. The original per-stream log decision is [0241](0241-each-topic-is-its-own-physical-log.md);
this record documents today's decision to retain the design.

```SQL
CREATE TABLE message_log (
  stream_id BIGINT NOT NULL,
  id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  payload JSONB NOT NULL,
  PRIMARY KEY (stream_id, id)
) PARTITION BY LIST (stream_id);

 CREATE TABLE message_log_17
  PARTITION OF message_log FOR VALUES IN (17)
  PARTITION BY RANGE (id);
```

## Decision

We retain separate per-stream tables after today's comparison with LIST/RANGE
subpartitioning, for these reasons:

- **Shared sequence ids** Because we went with id based range claim strategy ie
  [1, 100], [101, 200] etc. If we were claiming on a stream with low traffic and
  a seperate high traffic stream filled the vast majority of sequence ids. The low
  traffic stream would be progressing through potentially many empty ranges and this
  is only made worse with more streams.
- **Shared exclusive locks on initial plans and destroy table operations** Partition 
  drops claim an ACCESS EXCLUSIVE lock on parent table. This would be fine for TTL 
  based drops for RANGE subpartitions but for stream destruction which drops the LIST
  partition this would cause an temporary ACCESS EXCLUSIVE lock on main table. Now
  it is important to note that queries that directly target LIST partitions would be
  mostly fine. However a single parent table is a single source where multiple LOCKing 
  operations could compete and be a problem. Same thing for planning queries that aquire 
  small READ locks
- **Risker upgrade and migrations** It would be an all or nothing kind of thing here
  which has benefits but introduces a lot more risk as well.
- **Generally adverse to mixing what should be isolated resources**

## Consequences

Direct SQL still requires resolving the stream id and selecting its table name.
Agent assistance can reduce that effort, but manual querying keeps this cost.
We accept that cost to retain independent resources; the tradeoff may warrant
reconsideration if direct-query needs change.
