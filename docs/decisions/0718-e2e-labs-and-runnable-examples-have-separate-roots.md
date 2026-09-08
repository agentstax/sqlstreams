---
status: accepted
date: 2026-09-08
phase: pre-v1
---

# 0718 -- End-to-end labs and runnable examples have separate roots

## Context

`examples/phase_1` mixed executable verification labs with the producer and
consumer examples under `examples/playground`. The phase name no longer
described the long-lived role of the labs, and one module boundary covered
two different kinds of program.

## Decision

End-to-end verification and support programs live under `.e2e/`; runnable
user examples live under `.example/`. Each root is a dev-only nested module,
excluded from the library's published zip and root test surface. The lab
module owns its shared `common` package, and the root Justfile remains the
command surface for the labs.

## Consequences

Lab paths lose the phase prefix: `examples/phase_1/reclaimlab` becomes
`.e2e/reclaimlab`. Playground paths become `.example/01-produce-only` and its
siblings. The workspace and whole-repository verification command include
both modules independently.
