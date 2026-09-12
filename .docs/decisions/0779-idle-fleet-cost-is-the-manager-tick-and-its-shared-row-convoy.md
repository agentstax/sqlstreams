---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# The idle fleet's cost is the manager tick, and its losing claims convoy on the shared system rows

## Context

ROADMAP asked for an idle-fleet benchmark before any fix, then a rung on
the ladder: row `poll_rate`; idle backoff in the tick runner; LISTEN/NOTIFY.
The `idle-fleet-16/160/1600` family under `.bench` declares one group per
stream and produces nothing; the observer samples `pg_stat_statements` per
statement shape (pg_stat_statements drops the leading owner comment) and
the checker prints each shape's calls/s and share of statement time.

Read on 2026-09-12 (8-core Postgres 18.4, ten-minute holds):

| streams | rows | replicas | statements/s | Postgres cores | losing claim |
| --- | --- | --- | --- | --- | --- |
| 16 | 126 | 1 | 1,134 | 0.20 | 99/s, 0.02 ms |
| 16 | 126 | 3 | 2,763 | 0.31 | 259/s, 0.02 ms |
| 160 | 990 | 1 | 9,125 | 0.62 | 814/s, 0.01 ms |
| 160 | 990 | 3 | 24,153 | 1.65 | 2,275/s, 4.1 ms, 92% of statement time |
| 1600 | 9,630 | 1 | 7,356 | 8.1, saturated | 213/s, 31 ms; fleet never fully registered |
| 1600 | 9,630 | 3 | 3,649 | 7.7, saturated | 115/s, 800 ms; fleet never fully registered |

Every Consume session holds an uncapped group-scoped `manager` row that
ticks every second: a chain listing, two sweep DELETEs, and for each
capped row on its chain another process holds, a schema assert,
`SELECT ... FOR UPDATE`, `INSERT ... SELECT count`, and a rollback. The
chain carries three system-scoped rows (consumer group janitor, schedule
producer, metrics collector), so every group manager re-locks the same
three rows each second. Row `poll_rate` paces only a winner's work ticks.

## Decision

- Rung 1 is rejected by measurement: it does not reach the term.
- Rung 2 is the pick: a tick that made no progress backs off toward ten
  times the row's `poll_rate`; any progress snaps it back. A manager
  tick's progress is a reconcile change or a swept row. It divides the
  CPU term by about ten and moves the convoy's onset from about 480
  managers to about 4,800.
- Beside rung 2, a losing claim stops taking the row lock: it reads
  `target_instances` and the live count first and takes `FOR UPDATE`
  only when there is room; a stale "full" costs one tick. A shorter chain
  for the group manager (system rows left to the system manager) is the
  deeper fix and stays open.
- Rung 3 is not earned.

## Consequences

- One replica with a thousand rows idles at 0.6 Postgres cores today; ten
  thousand rows do not start. The fix ships as its own item.
- The family and the statement column stay on every scenario's report.
- CONVENTIONS "The literal" credits pg_stat_statements with attribution
  it cannot do; an inline marker past the first token would. A rule-file
  decision, not taken here.
- **Rejected:** rung 1; a knob on the manager row's `poll_rate`.
