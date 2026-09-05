---
status: accepted
date: 2026-09-05
phase: "pre-v1"
---

# Built-in alert config names the alert, not its scheduled work

**Context.** `RegisterSystemConfig.PartitionCount`, `CompactionReadCost`, and
`WorkerLiveness` named conditions without saying what was configured. At the
call site, `PartitionCount` looked like a system capacity setting. Their
`*JobConfig` types and `Expression` fields recovered the missing meaning by
exposing the internal scheduled-work mechanism.

**Decision.** The fields become `PartitionCountAlert`,
`CompactionReadCostAlert`, and `WorkerLivenessAlert`. Their types become the
matching `*AlertConfig`, and `Expression` becomes `ScheduleExpression`. The
flat fields remain: they keep every built-in alert directly discoverable from
`RegisterSystemConfig` without adding a grouping type that owns no new concept.

**Consequences.** A declaration reads as the thing configured and the precise
setting applied to it. These pre-v1 names replace the old public names without
compatibility aliases; implementation-local scheduled-work names are
unchanged.
