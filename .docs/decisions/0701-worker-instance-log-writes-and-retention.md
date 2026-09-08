---
status: superseded
date: 2026-09-07
phase: "pre-v1"
---

# Worker instance log writes and retention

## Context

Timestamp names are superseded by
[0702](0702-instance-log-timestamps-follow-existing-names.md).
Write and retention behavior below remains in effect.

[0700] approves ordinary instance snapshots as manager lease evidence.
This preliminary implementation adds persistence and cleanup only; interval
reads and collector-progress evaluation remain separate work.

## Decision

- The baseline creates worker_instance_log beside worker_instance, with its
  own id and recorded_at, copied worker_instance_id, worker_id, token,
  expires_at, attempts, and instance_created_at.
- Claim and renewal append the full snapshot after the live-row mutation in
  the same transaction. Declined claims and lost renewals append nothing.
  Logging failures roll back the mutation and use the existing retry path.
- There is no foreign key to worker_instance. Release and expiry cleanup
  leave history intact. The worker_config foreign key cascades worker deletion,
  matching worker_config_log. System deletion drops the new table explicitly.
- ManagerConfig.InstanceLogTTL defaults to 24h and must be positive.
  Existing manager refresh calls the history sweep after live-instance cleanup.
  Delete snapshots whose recorded expires_at is older than now minus this TTL.
  Index expiry for cleanup and worker_id/id for a worker's snapshot history.

## Consequences

Retention runs from recorded lease expiry, not log insertion, so a still-valid
lease snapshot cannot age out. Every renewal adds a transactional write.
Attempts are copied at claim/renewal time; this is not a success/failure log.
The baseline change requires existing development installations to recreate
or re-register the system tables before running workers with this build.
The targeted PostgreSQL test verifies rollback, retained history, cleanup, and
system deletion; alert behavior and liveness interpretation remain unchanged.
