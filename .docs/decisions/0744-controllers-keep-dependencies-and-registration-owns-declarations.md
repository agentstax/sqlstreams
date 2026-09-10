---
status: accepted
date: 2026-09-10
phase: "pre-v1"
---

# Controllers keep dependencies and registration owns declarations

## Context

[0742](0742-maintenance-settings-and-operations-are-separate.md) assembled
per-stream provisioners by rebuilding a StreamController during registration.
Stream and system controllers stored declarers in their constructors, unlike
other domain controllers. Worker declarations also bypassed RegisterWorker
because its sparse config changed an explicit zero target into one.

## Decision

Revise [0742](0742-maintenance-settings-and-operations-are-separate.md) in
registration assembly and initial-target handling; retain the maintenance
operations and the deferrals in [0743](0743-maintenance-completion-tracking-and-startup-delay-are-deferred.md).

StreamController and SystemController constructors take only their datastore
and logger. Their registration methods register their own resources. Admin
reuses those controllers and declares the workers afterward, preserving the
existing order and partial-failure behavior. Fixed system declarers belong
to admin; stream-specific provisioners are assembled from that call's config,
as the metrics collector already is. No shared controller is reconfigured.

RegisterWorker takes initialTarget as an explicit required InstanceTarget.
WorkerConfig holds optional metadata only. DeclareWorker checks definition
ownership, then uses RegisterWorker's shared validation and persistence path.
Zero starts a new worker suspended; registration never overwrites an existing
operational target. Consumer declarations explicitly keep NoInstanceTarget.

## Consequences

Other controller constructors already hold long-lived dependencies and need
no corresponding change. The supported client API is unchanged. Advanced
callers of the two constructors and RegisterWorker must adapt their calls.
Database writes and their ordering are unchanged; a failed declaration still
returns an error and can be retried by registering again. No schema change.
