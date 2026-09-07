---
status: superseded
date: 2026-09-07
phase: "pre-v1"
---

# All existing alerts share retained history

Superseded by [0699](0699-alert-timing-fields-are-inline.md) for configuration
shape; collector ownership and shared history behavior remain in effect.

## Context

The user required consistent pending support across partition count, compaction
read cost, and worker liveness before diagnostics or OTel work. Scalar metrics
needed same-observation context for compaction applicability and worker names.
The user approved metadata without compatibility machinery or payload-size limits.

## Decision

- Measurement adds optional Metadata JSON, excluded from its message key and
  exported attributes. Existing constructors and scalar export semantics remain.
  Retained reads, metrics consumers, and CLI JSON expose the attached metadata.
- The collector attaches compaction status to each partition-count measurement
  from the same topic snapshot, including the metrics topic. Compaction read
  cost reads that single message; its duplicate live-read datastore is removed.
- The collector produces a topic-level unclaimed-worker count with matching
  worker names, owners, and target instances. Consumer-group workers count
  toward their topic. Healthy reads produce zero and an empty list; failed
  reads produce no observation. Do not retain worker configuration documents.
- Every condition maps retained measurements to condition results. The alert
  domain's pure EvaluateHistory applies one duration/freshness/gap calculation.
  It remains separate from the write controller, which depends on the producer.
  Every worker retains Evaluate -> Record, with the same Pending configuration.
- Missing required metadata is insufficient evidence; malformed JSON returns
  an error. Both preserve recorded alerts and feed existing failed-check counts.
  Worker findings use the newest sample's names, not a later live lookup.

## Consequences

All three existing alerts now support history-based pending; hourly cadence and
immediate, log-only registration warnings remain. Metadata increases retained
payload size but not metric label cardinality. OTel continues reading scalar
values and attributes only. No compatibility or size-limiting mechanism is added.
Targeted race/unit tests cover shared timing, condition metadata, collector
ownership, and schedule policy. Database integration and labs remain deferred.
Collector-progress alerting and read-only evaluation diagnostics remain separate
unfinished work; collector monitoring must be independently observed.
