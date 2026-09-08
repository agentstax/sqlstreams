---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Throughput work excludes the archived delivery consumer

## Context

The user clarified that deliveryconsumer is archived and must not be used
in this work. The reliability throughput scenario uses the active cursor
consumer. The prior change also patched deliveryconsumer and exercised it
through a new regression and the existing routing lab.

## Decision

Supersedes the scope of [0714](0714-consumer-observations-allocate-transaction-ids.md).
Keep its cursor transaction-bound fix, empty-claim commit, and idle behavior.
Remove its deliveryconsumer patch and new tests of that archived path.
Do not use the mixed cursor/delivery routing lab for throughput validation.
Record 0389 remains historical; this work does not revise its implementation.

## Consequences

The throughput measurements still describe the active consumer: none of the
runs invoked deliveryconsumer. Earlier fan-out test results are historical
and do not validate the final patch. The archived path's discovered defect
is left outside this work; there is no claim that the final patch fixes it.
