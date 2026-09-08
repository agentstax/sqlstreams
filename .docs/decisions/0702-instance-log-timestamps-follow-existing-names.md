---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Instance log timestamps follow existing names

## Context

The user asked that worker_instance_log follow existing log-table timestamp
names. This supersedes the timestamp names in
[0701](0701-worker-instance-log-writes-and-retention.md); its write and
retention contracts remain unchanged.

## Decision

- Keep created_at as the copied worker_instance creation timestamp.
- Use attempted_at for the successful claim or renewal transaction's timestamp,
  following the existing binding_config_log distinction between an original
  timestamp and an attempt timestamp.
- Remove the names instance_created_at and recorded_at. No values are derived
  differently, and expiry-based cleanup does not change.

## Consequences

Snapshot fields retain their source names. Fresh baseline creation and the
snapshot insert agree on those names; existing development tables using the
previous names require recreation or a local column rename.
