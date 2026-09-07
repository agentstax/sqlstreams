---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Alert timing fields are inline

## Context

The shared pending policy [0695] [0698] used a nullable nested config even
though every alert always has resolved timing values. The user approved
direct fields to match the surrounding config pattern.

## Decision

- PartitionCountAlertConfig, CompactionReadCostAlertConfig, and
  WorkerLivenessAlertConfig declare PendingDuration, MaximumGap, and
  DisablePending directly. Defaults remain two minutes, two minutes, and false.
- JobPayload carries those same flat fields. Remove AlertPendingConfig and
  its public alias. Each config defaults and validates its own values.
- The shared history calculation reads the consumed JobPayload. Window
  calculation remains PendingDuration + 2*MaximumGap, or MaximumGap when
  pending is disabled. Evidence, activation, and recording rules are unchanged.

## Consequences

Callers configure timing without allocating another object or handling nil.
This replaces the nested configuration shape in [0695] and [0698], not their
history or collector ownership contracts. No compatibility translation is added.
