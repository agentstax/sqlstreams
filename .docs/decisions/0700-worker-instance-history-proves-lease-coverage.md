---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Worker instance history proves lease coverage

## Context

Collector-progress evaluation must distinguish an overdue collector from a
stopped installation without resetting pending on every manager replacement.
The user rejected a separate progress-observation series and extra lifecycle
fields, accepting lease-sized uncertainty after graceful shutdown.
This replaces the independent observation-series choice in [0684].

## Decision

- Add worker_instance_log as ordinary append-only snapshots of instance rows.
  Append after successful instance creation and renewal in the same datastore
  transaction. Preserve source identity, creation time, and expiry; no operation
  discriminator, derived liveness field, or pending state is required.
- Reconstruct manager lease coverage from retained snapshots. Overlapping or
  touching coverage survives instance replacement; an uncovered gap breaks the
  pending span. Collector completion history supplies the progress evidence.
- Graceful release does not shorten historical coverage. A released or crashed
  instance counts through its last recorded expiry, never through cleanup time.
  The uncertainty is bounded per instance by its configured lease duration:
  currently 30 seconds by default, renewed every 15 seconds.
- Log rows survive live-instance deletion and are removed by TTL cleanup.
  Retention and reads must preserve evidence covering the evaluation window,
  including intervals crossing its start. Missing history is insufficient
  evidence, not proof of a stopped or healthy installation.
- No observed_completion_timestamp measurement or separate observation worker.
  Keep collector-progress checks in existing scheduling and alert recording.

## Consequences

An overnight gap cannot earn pending duration. A short shutdown within the
last recorded lease can preserve pending and allow an earlier post-restart
alert. Coverage proves valid leases, not successful manager execution.
Each recorded renewal adds a snapshot write and makes log persistence part of
renewal success. TTL, indexes, cleanup placement, and the evaluation integration
remain implementation work; no runtime change is made by this decision.
