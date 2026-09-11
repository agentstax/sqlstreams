---
status: accepted
date: 2026-09-11
phase: "pre-v1"
---

# Benchmark runs share configuration and output

## Context

Manager still rebuilt Compose images per repetition, translated role settings
through a second set of Compose commands, and created a directory separate
from the checker's timestamp directory. Native runtime defaults also lived
inside the lifecycle method instead of the caller's run configuration.

## Decision

- Build Compose images once before repetitions. Reuse the invocation's
  Compose project and images while recreating its containers and volumes
  between repetitions.
- Manager constructs one role argument list. Native invokes the binary;
  Compose invokes that binary in the service container. Only filesystem
  paths differ. Compose declares infrastructure, not scenario arguments.
- Manager chooses each run directory once. Checker requires that directory
  through `-run-dir`, writes its verdict and report there, and retains
  measurements under its records directory. It creates no timestamp directory.
  Existing run indexes remain under results/<scenario>/runs.jsonl.
- Native roles inherit Go runtime environment settings. Manager injects no
  GOMAXPROCS, GOGC, or GOMEMLIMIT defaults. A comparison requiring the former
  4/400/2GiB settings supplies them explicitly; fingerprints record the
  supplied environment and role logs report the effective runtime.
- Keep [0753](0753-manager-coordinates-existing-benchmark-roles.md)'s shared
  role lifecycle, supplied native database ownership, CPU measurements, and
  failure cleanup. The fixed startup pause is outside this deduplication.

## Consequences

Repetitions use the same build, scenario settings have one translation, and
logs and verdicts identify the same output directory. Standalone checker
invocations must supply a run directory. Historical results and frozen
publication commands are unchanged. This supersedes [0753]'s native runtime
preset and establishes the single output directory for its lifecycle.
