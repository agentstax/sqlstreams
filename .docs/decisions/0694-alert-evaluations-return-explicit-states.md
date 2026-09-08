---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Alert evaluations return explicit states

## Context

Evaluate returned an alert or nil, and Record treated nil as healthy. Pending
and insufficient evidence must not resolve a recorded active alert. The user
approved an explicit result provided every existing check uses it consistently.

## Decision

- All three condition controllers implement Evaluator with AlertEvaluationResult:
  State is healthy, pending, active, or insufficient_evidence. Finding describes
  an unhealthy condition and is present only for pending and active results.
- Every scheduled worker keeps Evaluate -> Record. Record validates the result;
  pending and insufficient evidence return nothing without reading or writing
  the alert head. Healthy and active use the existing locked classification and
  production transaction. Only healthy can request recovery.
- Registration warnings read the same result and log findings for pending or
  active conditions. They remain immediate and log-only.
- These states are not new AlertStatus values or persisted evaluation state.
  Metrics retains ownership of evidence collection under [0693].

## Consequences

The shared result is used immediately by every existing evaluator and caller.
Missing partition measurements still return errors through existing check-failure
reporting. History-derived duration, freshness, and pending config remain the
next implementation work; this change does not enable them or change cadence.
Alert-lab repair and execution remain deferred at the user's request.
