---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Metrics collection owns alert evidence

## Context

The user rejected alert-owned measurement creation and storage. Moving it
between alert methods missed the existing metrics collector, which already
reads snapshots and produces measurements to __system.metrics.

## Decision

- Supersede [0692](0692-alert-evaluations-carry-findings-and-evidence.md).
  Metrics collection owns measurement production; alerts read collected
  measurements and Record owns alert transitions only. Remove AlertEvaluation
  and measurement production from the shared alert controller.
- Collect partition counts through the existing topic snapshot and collector,
  using the normal topic metric name and topic attribute. Expose Partitions
  beside Compacted on TopicMetricsHandle. Collect the metrics topic's partition
  count too, while continuing to exclude its traffic-dependent measurements.
- Partition-count Evaluate reads the retained metric through metrics and
  derives the live default threshold as before. Missing evidence returns an
  error, never a healthy result. Registration warnings use the same read path.
- Retain [0690]'s storage-time clock. Pending duration and freshness-window
  evaluation remain unfinished. Adapt other alerts after metrics collects the
  evidence they require; collector-progress monitoring must be independent.

## Consequences

Collection proceeds without running alert checks, and alert writes do not
create metric samples. Before the first collection, partition-count evaluation
reports missing evidence. It currently uses the latest retained count without
a freshness cutoff. Topic rename/recreation follows existing name-keyed metrics
semantics. Alert-lab updates and runs are deferred at the user's request.
