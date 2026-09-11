---
status: accepted
date: 2026-09-11
phase: "pre-v1"
---

# Benchmarks retain recurring workloads and freeze completed investigations

## Context

The root lab now owns benchmark execution. The user approved auditing each
workload's value rather than migrating every historical experiment.

## Decision

- Keep quiet for steady delivery and max-throughput for sustained capacity.
  Keep the multi-stream family experimental: its group and instance counts
  grow with stream count, so it compares deployments rather than one variable.
- Merge dev into a one-minute quiet invocation, `just reliability-smoke`.
  Require a scenario for `just reliability-lab`; retire throughput's short
  fixed-rate declaration. Report mode reads historical results by name.
- Retire independent claim, compaction, fillfactor, scale, trigger-fanout,
  and native scratch programs. Freeze their tracked source together in
  `.bench/results/retired-source-2026-09-11.tar.gz`, preserving original paths.
- Keep historical results, design rationale, and local investigation evidence.
  Consumerdefaults and idempotency already retain results without a tracked
  runner. Do not recreate their temporary tuning harnesses.
- A maintained scenario names a recurring decision, distinct measurements,
  and the action a changed result would prompt. Completed optimization
  experiments do not automatically become maintained scenarios.
- Retain existing correctness tests. The compaction hot-key deadlock assertion
  lives in compactiondeadlock e2e; stale commit and cursor-floor invariants have
  integration tests. Sharded prototype assertions describe unshipped machinery.
  Scratch identity checks remain frozen evidence, not a claim that aggregate
  max-throughput now proves identities after retention.

## Consequences

No new workload engine or fault mechanism is introduced. Historical report
names remain readable; new smoke runs record under quiet. Idle-fleet costs,
automatic-batching capacity, and recovery under load remain separate research
questions. Historical source is available without being maintained as live code.
