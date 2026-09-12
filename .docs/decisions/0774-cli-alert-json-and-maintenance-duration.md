---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# CLI alert JSON and maintenance durations follow the read contract

## Context

Alert latest and history returned the same name/exists/alerts envelope despite
the client's distinct object and array results. Maintenance status serialized
UnclaimedFor as nanoseconds while CLI durations otherwise carried units.
Scheduler table headings and one registration error used different terminology
from the client fields and verbs.

## Decision

- Alert latest JSON is the alert object or null; history JSON is the newest-first
  array or []. Preserve exit 1 for no retained alerts in both commands.
- Maintenance status keeps the worker snapshot's fields and renders unclaimed_for
  as a duration string, including "0s". Use the existing CLI result-document pattern.
- Scheduler tables spell Expression and ConsumerGroup as EXPRESSION and
  CONSUMER_GROUP; the system error says registered instead of initialized.
- Keep command and flag names, metric series filtering, and worker config reads.

## Consequences

Pre-v1 scripts must read alert latest fields directly instead of .alerts[0],
and history entries at .[] instead of .alerts[]. Maintenance readers must parse
unclaimed_for as a duration instead of dividing nanoseconds. Exit codes still
distinguish empty alert reads; the library and database behavior are unchanged.
