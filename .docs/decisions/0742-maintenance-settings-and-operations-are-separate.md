---
status: superseded
date: 2026-09-10
phase: "pre-v1"
---

# Maintenance settings and operations are separate

## Context

[0741](0741-vacuum-settings-follow-stream-declarations.md) made vacuum
registration replace operational targets. That contradicted the existing
rule that scaling and suspension survive application restarts. Vacuum also
had a public config without a corresponding janitor config.

Superseded in completion tracking and initial scheduling by
[0743](0743-maintenance-completion-tracking-and-startup-delay-are-deferred.md).

Registration assembly and initial-target handling are revised by
[0744](0744-controllers-keep-dependencies-and-registration-owns-declarations.md).

## Decision

Supersede [0741](0741-vacuum-settings-follow-stream-declarations.md).
StreamConfig carries parallel JanitorConfig and VacuumConfig declarations.
Admin assembles their declarers through the existing stream registration
path. Registration updates metadata and always preserves existing targets.
New streams declare janitor target one and vacuum target zero; definitions
accept explicit zero while sparse WorkerConfig keeps its existing defaults.

StreamHandle.Janitor and Vacuum return MaintenanceHandle with Suspend,
Unsuspend, and Status. These resolve the named stream and worker through
admin. Controller ForceUpdateTargetInstances is a separate operation that
changes and audits the target atomically, preserving metadata. Repeating
the current target makes no new history row.

Suspension prevents new claims. Running instances observe zero on heartbeat,
cancel work, and release their claims normally; suspension never deletes a
live claim early. Unsuspend permits one maintenance instance. Changes to
settings take effect when a new instance claims the worker.

Status reuses worker snapshots, reporting suspended, unclaimed, claimed,
or failing. Successful janitor and vacuum passes append completion time to
worker_instance_log; heartbeats do not. An indexed lookup returns the latest
retained completion even after the instance releases. Other tick workers
retain their existing record-on-recovery behavior.

## Consequences

Users enable scheduled vacuum with orders.Vacuum().Unsuspend(ctx). Putting
that operation in every startup path would override operator suspension.
Status exposes pending shutdown through live-instance counts and actual
completion separately from liveness. History follows existing retention.

Vacuum stays optional and uses the existing manager, independent pacing,
timeouts, jitter, retries, and pool. Autovacuum stays enabled. Each completed
maintenance pass adds an instance update and history row; the produce and
consume paths are unchanged. The pre-v1 baseline gains a nullable completion
column and index, so existing development databases need recreation.
