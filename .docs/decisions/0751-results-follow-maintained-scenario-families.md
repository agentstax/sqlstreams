---
status: accepted
date: 2026-09-11
phase: "pre-v1"
---

# Results follow maintained scenario families

## Context

The user removed the historical experiment archive and clarified that results
for current scenario families should remain. This supersedes
[0750](0750-benchmarks-retain-recurring-workloads.md)'s evidence-retention policy;
its active workload selection and retired executable decisions continue.

## Decision

- Keep quiet, max-throughput, older multitopic results belonging to the current
  multistream family, and published throughput evidence.
- Rename the dev result directory to quiet. Preserve original record contents,
  including declarations, durations, scenario names, and measurement identities.
  Reports select the containing directory; new quiet runs append there.
- Delete throughput exploration results: short automatic-batching tuning,
  distinct from the maintained explicit-batch max-throughput workload.
- Delete check-* and _controls: checker-validation and investigation artifacts,
  not maintained workload histories. Delete retired-source, fingerprint, and
  statistics scratch files as well.
- Do not recreate the historical experiment results the user deleted.

## Consequences

Kept results follow scenario families even when historical names differ.
`just reliability-report quiet` includes former dev records without rewriting
those records as hour-long measurements. Retired experiment results are no
longer available from the working tree; current documentation reflects that.
