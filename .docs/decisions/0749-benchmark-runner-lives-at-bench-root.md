---
status: superseded
date: 2026-09-11
phase: "pre-v1"
---

# Benchmark runner lives at the .bench root

Superseded by [0755](0755-benchmark-commands-and-runs-use-bench.md) for command
naming; the root layout and evidence preservation continue unchanged.

## Context

The reliability lab is the shared running system for benchmark scenarios.
Its extra directory puts that system beside older independent experiments.

## Decision

- Promote the lab packages, entry point, scripts, Compose stack, and results
  from `.bench/reliability/` to `.bench/`.
- Keep `just reliability-lab` and `just reliability-report` as the commands.
- Preserve recorded evidence bytes and frozen source layouts; update current
  evidence links. Historical reproduction commands target the frozen source.
- The evidence retention policy in [0748](0748-published-throughput-uses-three-fixed-duration-runs.md)
  continues at `.bench/results/published/`.
- Classifying or migrating the other benchmark programs is separate work.

## Consequences

New scenarios extend the existing root runner. Imports, Docker build paths,
and native source capture follow the shallower directory. Existing workloads
and verdicts are unchanged; old result paths in historical records describe
where those artifacts lived when recorded.
