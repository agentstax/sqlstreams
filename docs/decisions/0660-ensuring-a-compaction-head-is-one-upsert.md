---
status: accepted
date: 2026-09-05
phase: "pre-v1"
---

# Ensuring a compaction head is one upsert

Supersedes the transactional lock-acquisition mechanism in [0659]. Its
lockable-row, TTL, observability, and public-API decisions still stand.

**Context.** [0659] selected an existing row `FOR UPDATE`, refreshed an empty
row separately, and inserted a missing row with `ON CONFLICT DO NOTHING` before
repeating after a lost race. That preserves the invariant but takes multiple
statements and carries resolution logic solely to distinguish those cases.

**Decision.** Ensure and lock the row with one statement:

```sql
INSERT INTO compaction_head_<topic_id> AS h (compaction_key)
VALUES ($1)
ON CONFLICT (compaction_key) DO UPDATE
SET updated_at = CASE
    WHEN h.head_id IS NULL THEN NOW()
    ELSE h.updated_at
END
RETURNING compaction_key, head_id, schema_version, compaction_rank,
          created_at, updated_at;
```

The insert owns a new row; the conflict update locks an existing row until the
transaction resolves. The conditional expression refreshes a headless row and
preserves a populated row's timestamp. `RETURNING` gives both cases the same
result shape, removing the absence race and application loop.

**Consequences.** Lock acquisition is one database round trip. A materialized
head still needs its message-log lookup. PostgreSQL creates a new row version
on every conflict, including the populated-head case where the assigned value
does not change; this is the accepted cost of a single statement that also
returns populated heads. A conflict-update `WHERE h.head_id IS NULL` was
rejected because a populated row would be locked but absent from `RETURNING`.
