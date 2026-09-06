---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Client results remain unnamed

## Context

The named-return-params roadmap task considered naming every result as
signature documentation. A trial covered nine client, topic-handle, and
producer functions, with explicit return expressions retained. Review through
the playground call sites showed that the client design already makes the
results clear: the verb and concrete return type identify what comes back.

## Decision

- Revert the nine trial signatures and retain unnamed results by default
  on the client, handles, and instances, including constructors.
- Do not standardize named results as an additional documentation layer;
  in this API they repeat information without resolving ambiguity.
- Keep return expressions explicit. This decision does not ban named
  results needed by an implementation, such as deferred result handling.
- Close the roadmap task without a repository-wide signature sweep.

## Consequences

The playgrounds and runtime behavior are unchanged. Existing named results
elsewhere are untouched. CONVENTIONS.md records the client-surface default;
mandatory named results should not be proposed again without new evidence
of ambiguity in that surface.
