---
status: superseded
date: 2026-09-10
phase: "pre-v1"
---

# Vacuum settings follow stream declarations

## Context

Review found that [0740](0740-scheduled-key-vacuum-is-opt-in.md) required SQL
edits instead of the public configuration-to-declaration path used by the
metrics collector. Worker registration traditionally preserves targets,
which would prevent Go config from enabling a previously disabled vacuum.

Superseded by [0742](0742-maintenance-settings-and-operations-are-separate.md).

## Decision

Supersede [0740](0740-scheduled-key-vacuum-is-opt-in.md) in configuration
ownership. StreamConfig.Vacuum declares Enabled, PollRate, and VacuumTimeout.
Defaults remain disabled, two minutes, and one minute. Admin assembles the
provisioner from the resolved config and declares it after stream registration.

WorkerConfig.ReplaceTargetInstances explicitly applies the target on
re-registration. Vacuum uses it; other workers preserve their existing
behavior. Target and metadata changes share the registration transaction
and worker_config_log history. Controller validation remains in both entry
paths, including explicit zero targets.

## Consequences

No SQL edit is needed to configure vacuum. Registrars for one stream must
agree on its settings. Existing instances keep their configuration until
restarted; registration controls newly claimed instances. Requests retain
independent scheduling, durability, autovacuum, and timeout behavior.

Every vacuum exit logs session duration. Datastore integration tests cover
vacuum and declaration history; a clock-controlled unit test covers initial
delay and slow ticks without a database. No new throughput benchmark.
