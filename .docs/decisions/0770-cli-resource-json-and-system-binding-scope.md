---
status: superseded
date: 2026-09-12
phase: "pre-v1"
---

# CLI resource JSON and binding scope follow the client

Superseded by [0771] for scheduler concurrency choices; the other decisions remain.

## Context

Metric --system selected built-in names across every scope. Consumer binding
list called System().Bindings. Stream and scheduler get wrapped their rows
under different keys while consumer get returned the resource directly.
Scheduler concurrency help omitted ordered; an alert example named no built-in.

## Decision

- Rename metric list --system to --builtin; keep --user as its exclusive
  alternative. The filter still selects the sqlstreams. name prefix.
- Move consumer binding list to system binding list for System().Bindings.
  Consumer binding get stays with Consumer(name).Binding().Get.
- Stream, scheduler, consumer, and system get return the resource object in
  JSON, or null with exit 1 for absence. Keep CLI duration strings and the
  client's field names; scheduler JSON includes schema_version.
- Remove the old command and flag without aliases; reject them with exit 2.
- Help lists parallel, exclusive, and ordered; alert examples use declared
  built-in names at their declared scopes.
- Supersede [0576]'s missing-get envelopes. Keep its output modes, error
  documents, duration rendering, quiet/JSON guard, and mutation summaries.
  The command and read-operation decisions in [0766] and [0768] still apply.

## Consequences

Pre-v1 scripts must use --builtin and system binding list. Stream JSON paths
such as .config.stream_id become .stream_id; scheduler .row.schedule_id
becomes .schedule_id. Absence is null plus exit 1 instead of exists:false.
System get now emits null on stdout for absence in JSON mode. Exit status
remains the absence signal; no alias silently changes a command's scope.
The existing command-path test changes because binding list intentionally
moves; selector validation keeps its coverage with a declared alert name.
