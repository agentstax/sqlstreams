---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Documentation starts with a runnable example and separates progress from delivery outcomes

## Context

The website review found missing Quickstart run instructions, table internals
before delivery concepts, and broad claims that hid normal deferred state and
dead deliveries behind cursor progress. Guides mixed configuration with claim
implementation, and wide text diagrams required horizontal reading.

## Decision

- Quickstart ends at a visible produce/consume result, with filenames,
  commands, connection setup, expected output, and cancellation behavior.
  Transactional writes and failure inspection remain linked follow-ups.
- Getting Started leads with Quickstart. Concepts teach group independence,
  lifecycle, and key policies before architecture and physical table design.
- Lifecycle distinguishes retries, delays, deferred state, and terminal
  outcomes. Cursor progress is read alongside unresolved and dead deliveries;
  retention limits are stated where retained failure state is explained.
- Guides lead with configuration and verification. Ordering and registration
  mechanics live in their concept pages. Existing page URLs are retained.
- Existing Markdown tables and causal steps replace the wide architecture
  and lifecycle text diagrams. No new component or documentation tooling.
- Orientation states the transaction benefit with its operating and
  at-least-once boundaries; implementation history stays in decision records.

## Consequences

The pass applies the page-kind split in [0679] without changing the library
or the site layout. The two complete tutorial programs can be extracted and
run against an isolated installation. Mechanism detail remains available,
but is no longer a prerequisite to following a configuration guide.
