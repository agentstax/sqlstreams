---
status: superseded
date: 2026-09-11
phase: "pre-v1"
---

# Manager coordinates existing benchmark roles

## Context

The native launcher already executes the lab's Go roles. Their POSTGRES_*
connection settings and scenario controls carry the workload without a
second implementation. Direct-role probes confirmed execution; providing
CPU samples also completed the existing checker. Translating the launcher's
publication and filesystem utilities into Go obscured that boundary.

## Decision

- Manager coordinates existing producer, consumer, observer, and checker
  processes through `-role manager -execution native|compose`. Runner still
  executes one role. The Just recipe builds the binary and passes settings.
- Native uses the supplied POSTGRES_* connection and requires an empty,
  durable database. It neither creates nor drops that database, and runs one
  repetition. Compose owns its temporary stack and supports repetitions.
  Both support multiple consumer processes and clean up owned processes.
- Observer samples local role CPU without inspecting the database host's
  filesystem. Checker still collects server settings and judges the run.
  Missing service CPU is null in JSON and unavailable in the report.
- Native runtime defaults remain GOMAXPROCS 4, GOGC 400, GOMEMLIMIT 2GiB.
  They are configurable Go settings, not OS resource quotas. Native CPU
  headroom uses host capacity; Compose retains container CPU quotas.
- Remove native.py, its source copying, storage scans, configuration dumps,
  and repeated exception-worker inspection. Existing scenario registration
  and producer startup retain their exception-worker controls.

## Consequences

Connecting to an external server needs no new connection implementation or
PostgreSQL binary installation. Native repetitions require a fresh supplied
database; old run data cannot silently contaminate the next run. Server CPU
is unavailable without separate host measurements, while SQL measurements
remain available. WAL and checkpoints still require a dedicated server for
attribution. Filesystem budgets and publication archives are separate work.

This supersedes the native database-ownership and launcher requirements in
[0745](0745-maximum-throughput-uses-the-reliability-lab.md). Frozen published
source and evidence keep their original reproduction procedure.

Runtime configuration and output ownership are superseded by
[0754](0754-benchmark-runs-share-configuration-and-output.md); its other
role and database lifecycle decisions remain in force.
