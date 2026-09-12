---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# CLI commands mirror client verbs

## Context

CLI reads mixed current values, history, and status under get. Passing
--limit changed an alert read into history, while schedule messages were
a flag and key messages a command. Schedule also differed from Scheduler.

## Decision

- Use scheduler for the client's Scheduler handle; get, status, and messages
  invoke their corresponding methods separately.
- Metric and alert reads use latest and history; only history accepts --limit.
- Key compaction-head names CompactionHead; messages keeps Messages.
- System register names System().Register and applies default system config.
- Metric --series-limit bounds attribute sets; --limit bounds entries.
- Keep collection plurals: messages, migrate streams, migrate versions,
  and the Prometheus --metrics-address endpoint.
- Remove the former spellings without aliases. Update help, examples, and
  error recovery commands together.

## Consequences

Pre-v1 scripts must update command paths and metric --series. Scheduler get
no longer includes consumer_groups or messages in its JSON document; status
and messages each return their own array. System register returns registered
rather than a hard-coded migration version. Removed flags and command paths
are rejected, making stale invocations visible instead of changing their read.
