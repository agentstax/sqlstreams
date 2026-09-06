---
status: accepted
date: 2026-09-05
phase: "pre-v1"
---

# The janitor timeout names cleanup

## Context

The timeout field in [0662] named an implementation step rather than the
cleanup operation a caller wants to limit.

## Decision

Rename `JanitorConfig.SweepStepTimeout` to `JanitorConfig.CleanupTimeout`
and its start-log attribute to `cleanup_timeout`. The field comment states
that each cleanup operation gets its own timeout, including queries and
retries. This supersedes only the naming in [0662].

## Consequences

The five-second default, validation, separate deadlines, cancellation, and
error collection are unchanged. Callers use the new field name.
