---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# The checker drains on the consumer group cursor, and every scenario declares the safety checks

Supersedes [0687]'s drain clause only; the rest of that record stands.

## Context

[0687] had the reliability lab's consumers drain "until each producer's
last committed key has a delivery outcome". Building it showed the
criterion is not sufficient: instances hold ranges in parallel, so the
last id finishing says nothing about an earlier range still in flight,
and the checker would read that range as undelivered -- a false FAIL in
a suite whose one job is a trustworthy verdict. Making the criterion
sound means also waiting for "no open lease, no unresolved exception
row", which is exactly what the cursor advancer computes to move
`consumer_group_cursor.committed`.

[0687] also listed the safety checks (lost, unexpected, undelivered,
unbucketed, duplicates, recovered) as the `[expect]` lines a scenario
declares. Declared means optional: a scenario that omits `lost` runs
green while checking nothing.

## Decision

- Drain is `committed >= MAX(message_log.id)` for the group, polled
  under a budget; the budget spent is a verdict of unknown. The topic's
  highest id, not the ledger's last committed id, so a recovered or
  unexpected row above it is settled too. A scenario's consumer
  timeline must end with at least one instance running, since only a
  running consumer advances the cursor.
- `scenario.Invariants` names the six safety checks with their wants
  (lost, unexpected, undelivered, unbucketed 0; duplicates, recovered
  report). `Validate` rejects a scenario that omits one or wants it
  differently. They stay in the `.scenario` file for the reader; the
  rest of `[expect]` is the scenario's own (reclaims, dead, recovery).
- `undelivered` is witnessed by the handler ledger (the lab's handler
  succeeded, or the library dead-lettered); `unbucketed` by the
  library's own tables (a `success` delivery_log row or a `dead`
  exception row, exactly one). Two witnesses, two checks: a success the
  library records without the handler running fails the first; a
  message the library silently drops fails both.

## Consequences

- A cursor that advances past unfinished work cannot pass the run: the
  checks then read unfinished work and fail it. The drain trusts the
  mechanism under test only to end the wait, never to judge.
- `unbucketed` needs `DeliveryLogMode all`, so the lab does not yet run
  the default mode; the checker refuses any other mode with an unknown
  verdict. Judging the library's success bucket without delivery_log
  rows is open for the chaos scenarios, as is whether `dead` is
  witnessed by the ledger's error facts instead of the exception row.
