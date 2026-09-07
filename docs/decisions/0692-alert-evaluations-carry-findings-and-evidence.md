---
status: superseded
date: 2026-09-06
phase: "pre-v1"
---

# Alert evaluations carry findings and evidence

## Context

Moving partition-count observation and storage behind RecordMeasurement kept
a different execution pattern from the other alert workers. The user rejected
that alternate path and approved one Evaluate -> Record contract.

## Decision

Superseded by [0693](0693-metrics-collection-owns-alert-evidence.md):
measurements belong to metrics collection, not alert evaluation or recording.

- Continue [0689](0689-alert-implementation-starts-in-existing-recording.md)
  through the existing Evaluator interface. Evaluate returns AlertEvaluation,
  containing the condition's finding and measurements to retain.
- Every scheduled alert worker calls Evaluate and then Record. Evaluate owns
  source reads and comparison; Record owns evidence storage and the existing
  serialized alert transition. Remove RecordMeasurement, MeasurementEvaluator,
  and partition count's separate Observe/EvaluateMeasurement methods.
- Registration-time warnings inspect the same evaluation without calling
  Record. Healthy results have no Alert; evaluation failures return errors.
- Partition count retains raw measurements. The other conditions use the same
  contract without adding evidence shapes before their adoption review.
  Storage time remains the evidence clock under [0690].

## Consequences

The workers share one flow, with no measurement persistence details or alternate
recording method. Evidence survives a subsequent alert-write failure. This
change does not enable pending duration or change alert cadence; history-based
classification remains unfinished work inside Record.
