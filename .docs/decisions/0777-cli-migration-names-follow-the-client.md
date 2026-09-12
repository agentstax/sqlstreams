---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# CLI migration names follow the client

## Context

Migration commands called the client's targetVersion argument --to and
reported initialized despite the client's Register terminology. The naming
sweep also identified config comparisons and multi-series metric reads as
documented CLI conveniences beyond the client's resource reads.

## Decision

- All migration up/down commands require --target-version; remove --to
  without an alias. Help and recovery commands use the new spelling.
- Migration result JSON uses target_version; migration status uses registered
  in JSON and registration terminology in text.
- Keep stream config get's default/current comparisons and key selection.
- Keep metric latest/history's partial attribute filters, series-limit,
  and series JSON envelope. Document their difference from exact client reads.

## Consequences

Pre-v1 scripts must update --to, .to, and .initialized to --target-version,
.target_version, and .registered. Old flags exit 2 before connecting;
JSON readers must update their field selectors. Migration direction, target
validation, exit statuses, and database operations are unchanged.
The config and metric conveniences remain explicit exceptions to exact
client result shapes, with no change to selection or output behavior.
