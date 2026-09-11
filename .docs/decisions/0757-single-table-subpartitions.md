---
status: accepted
date: 2026-09-11
phase: pre-v1
---

# 0757 — Why not instead of per topic tables a single table with subpartitions?

## Context

Currently we create and manage a set of tables *_<topic_id> this causes users
who want to query directly to see live data to have to first lookup topic id
then put that value in table query DDL. This is obviously not great but there
is reasoning behind it. 

Example:

```SQL
CREATE TABLE message_log (
  topic_id BIGINT NOT NULL,
  id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  payload JSONB NOT NULL,
  PRIMARY KEY (topic_id, id)
) PARTITION BY LIST (topic_id);

 CREATE TABLE message_log_17
  PARTITION OF message_log FOR VALUES IN (17)
  PARTITION BY RANGE (id);
```

## Decision

It was decided to not do single table with LIST partition and RANGE subpartition
for these reasons:
- **Shared sequence ids** Because we went with id based range claim strategy ie
  [1, 100], [101, 200] etc. If we were claiming on a topic with low traffic and
  a seperate high traffic topic filled the vast majority of sequence ids. The low
  traffic topic would be progressing through potentially many empty ranges and this
  is only made worse with more topics.
- **Shared exclusive locks on initial plans and destroy table operations** Partition 
  drops claim an ACCESS EXCLUSIVE lock on parent table. This would be fine for TTL 
  based drops for RANGE subpartitions but for topic destruction which drops the LIST
  partition this would cause an temporary ACCESS EXCLUSIVE lock on main table. Now
  it is important to note that queries that directly target LIST partitions would be
  mostly fine. However a single parent table is a single source where multiple LOCKing 
  operations could compete and be a problem. Same thing for planning queries that aquire 
  small READ locks
- **Risker upgrade and migrations** It would be an all or nothing kind of thing here
  which has benefits but introduces a lot more risk as well.
- **Agentic querying makes finding the correct table trivial**
- **Generally adverse to mixing what should be isolated resources**

In summary: Logical isolation keeps things safer and easier to reason about but the
user cost is acknowledge and this could have been the wrong decision.
