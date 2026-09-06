---
status: superseded
date: 2026-09-05
phase: "pre-v1"
---

# Janitor cleanup steps have separate deadlines

## Context

The topic janitor passed its lifecycle context through five sequential cleanup
calls with no deadline. Batch limits did not bound database waits. Returning
the first error also prevented all later cleanup categories from running.

## Decision

Each existing controller call gets its own child context and immediate cancel
call. `JanitorConfig.SweepStepTimeout` defaults to five seconds, validates
positive, and includes the step's datastore retries. Keep all five calls
explicit in the instance; introduce no step registry or execution helper.

Collect errors with the cleanup operation attached, then return them together.
Check the parent context before each step; shutdown or claim loss ends the
pass and preserves errors already collected. Retain the DDL lock timeout.

## Consequences

A slow or broken cleanup category no longer prevents later categories from
being attempted. Earlier committed batches remain committed. Any step error
makes the existing tick runner record failure and back off. The start log
reports the timeout, and the combined error identifies each failing step.

A pass can consume five step budgets plus cancellation overhead and subsequent
runner bookkeeping. This does not guarantee fairness among partitions inside
one cleanup step. The managed fleet uses the default; direct provisioner
construction can override it. No migration is needed.

Naming superseded by [0663]; timeout behavior remains accepted.
