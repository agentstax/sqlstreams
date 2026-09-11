---
status: superseded
date: 2026-09-10
phase: "pre-v1"
---

# Maximum throughput uses the reliability lab with explicit workload and environment settings

## Context

Native scratch benchmarking established a paired steady-state reference.
The lab must reproduce that workload without replacing its records and
checker, weakening existing scenarios, or exceeding the 100GB storage budget.
This supersedes [0711](0711-reliability-lab-is-the-benchmark-harness.md).

## Decision

- Every Postgres benchmark remains a scenario in `.bench/reliability`, using
  its existing roles, records, checker, verdict, and append-only run ledger.
  CPU microbenchmarks remain Go benchmarks. No second measurement harness
  or histogram dependency is introduced.
- Preserve paced production and automatic batching. Explicit batches and
  unpaced phases are selectable; Rate 0 without unpaced still means idle.
  Unpaced production declares a bounded caller count.
- Stream declarations carry retention, janitor, and vacuum configuration.
  Scale declared TTLs and partial-sweep grace with the timeline; keep polls,
  operation timeouts, and leases fixed. Short runs are functional checks.
- Keep every attempted produce, outcome, and handler invocation. Measure
  recording overhead before changing representation or sampling. Budget
  raw files, PostgreSQL storage, and checker imports together.
- With retention, verify acknowledged messages even after their rows expire.
  A missing expired row needs successful handling and durable completion
  for every declared group. Without retention, missing rows still fail.
  This does not prove the exact instant an already-deleted row expired.
- The observer reads allocated ids and committed cursors without scanning
  message partitions. Label allocation as an upper bound, including gaps
  and uncommitted ids; exact completion still comes from message records.
- Warmup is explicit; it stays in safety checks and phase measurements,
  but run-level performance guards apply to measured phases.
- Schedule adherence applies to paced phases. Idle runs need no produced
  messages. Missing measurements cannot count as passing checks. Scenarios
  may strengthen a report-only expectation to zero, never weaken zero.
- `just reliability-lab` retains compose by default and gains native macOS
  execution of the same roles against an explicit server. Record execution,
  runtime, host, server version, and effective settings in result identity.
  Native runs own a disposable database and clean it up afterward.

## Consequences

- Maximum throughput reports production and consumption separately, with
  backlog trends over the measurement phase. An opening burst or successful
  final drain alone cannot establish steady-state capacity.
- The raw ledger costs about 82GB for 30 minutes at 65k/s, before database
  and checker space. Begin with shorter runs under a total storage guard.
- Chaos, keyed compaction, and idle-fleet scenarios retain their separate
  requirements; this change does not implement their deferred behaviors.

Superseded by [0747](0747-throughput-scenarios-can-disable-message-recording.md) for optional aggregate-only recording;
other workload and environment decisions remain in force.

The native database-ownership and launcher requirements are superseded by
[0753](0753-manager-coordinates-existing-benchmark-roles.md).
