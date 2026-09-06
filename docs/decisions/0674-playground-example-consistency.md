---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Playground examples share handle, instance, and lifecycle patterns

## Context

Topic reuse, handler placement, lifecycle setup, and variable roles varied
between playground examples without a scenario-specific reason.

## Decision

- Create each topic handle once and reuse it. Retain registration results
  only when an operation needs their fields, such as schedule registration.
- Name handles for their domain and instances for their activity. Qualify
  instance names when several of the same kind exist.
- Always declare consumer handles separately from their registered instances.
- Put consumer handlers in named functions below run.
- Use LifecycleContext with deferred stop in every example and observe
  cancellation during simulated waits.
- Use routines and routinesCtx for errgroup orchestration.

## Consequences

The examples teach one structure while retaining their different scenarios
and topic-registration prerequisites. Handler placement and explicit consumer
handles are the current choices; the user may revisit them later.
CONVENTIONS.md holds the binding rules. The finite examples now respond to
shutdown signals, and the metrics handler's simulated wait is cancellable.
