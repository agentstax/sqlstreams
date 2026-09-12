---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# The group manager keeps its stream's upkeep and drops the system rows

## Context

[0779] moved the three system-scoped rows (consumer group janitor,
schedule producer, metrics collector) off the group manager's chain and
left open what a consumer-only process with `ClientConfig.DisableManager`
keeps alive. Tracing the chain settled two facts. The group manager's
provisioner list also carries the stream janitor and stream vacuum, so
today every consumer process runs DDL for its own stream, and the client
and manager pages' claim that `DisableManager` suits a database role with
no DDL rights has been false since the stream rows joined that list.
And in the 160-stream cell, 480 of the 814 losing claims a second were
group managers on the three system rows; the other 320 were the system
manager on the stream janitor and cursor advancer rows the group managers
already held.

Two lines were weighed. Dropping the stream rows too would make the group
manager reconcile only its group's rows and make the DDL claim true, but
every stream's janitor would then run in whichever process holds the
system manager row: one pod carrying the deployment's retention work
reads as a fault to an operator, not as upkeep running correctly.

## Decision

- The group manager reconciles its group's rows and its stream's rows:
  message consumer, exception consumer, cursor advancer, stream janitor,
  stream vacuum. The three system-scoped rows leave its list; the system
  manager, one winner per deployment, reconciles them.
- A `DisableManager` consumer therefore keeps alive its group's consumers,
  its cursor advancer, and its stream's janitor and vacuum, and nothing
  deployment-wide: the schedule producer, metrics collector, consumer
  group janitor, and alert checks need a system manager somewhere. A
  consumer's own Register warns with SQL0063 while such rows sit
  unclaimed.
- The client and manager pages drop the "no DDL rights" reason and state
  the rule: the consumer's own stream janitor runs in the consumer
  process, so its database role needs DDL rights on that stream's tables.
  [0635]'s accepted-risk clause is history and stays as written.

## Consequences

- Retention work stays spread over the consumer pods that own the
  streams; only three rows move.
- The system manager still loses one claim a second per held stream
  janitor and cursor advancer until [0779]'s idle backoff and lock-free
  losing claim land beside this.
- **Rejected:** dropping the stream rows from the group manager; a
  `DisableManager` that leaves DDL to another process.
