---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Export validation checks only export compatibility

## Context

Supersedes [0706]. Export validation repeated constructor checks and indexed
output names from families already rejected. That added bookkeeping and
omitted healthy families for collisions that could not reach the output.

## Decision

- Core constructors own kind/unit validity and built-in declaration identity;
  the custom producer owns the raw vulkan. namespace restriction. Do not
  repeat these checks in the adapter. This change adds no new write checks.
- Keep export-specific checks: family-wide kind/unit consistency, translated
  attribute-key ambiguity, translation failures, and reserved output names.
  Retained measurements still cannot supply collection-health gauges.
- Translate each eligible family once. Check name collisions only among
  families that passed their own export checks; reject all participants in
  a remaining collision, independent of row order.
- Keep [0706]'s collection-local read-health and rejection gauges, logging,
  error behavior, pinned translation strategy, and reader contract unchanged.

## Consequences

A rejected family remains logged and counted but no longer blocks an otherwise
healthy family from exporting. The adapter trusts core measurement validity;
it is not a backstop for measurements mutated after construction or written
outside the intended producers. No cache or persistent rejection state is added.
