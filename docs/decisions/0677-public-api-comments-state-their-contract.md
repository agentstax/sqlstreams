---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# 0677 — Public API comments state their contract

## Context

The doc comments a caller reaches through `vulkan` never had a pass after
the client redesign [0625]; [0664] folded the audit into one review and
[0670] settled the surface it covers. A sweep of the alias closure found
about ninety findings: stale names for verbs that no longer exist, a
handful of claims the code contradicted (MessageOptions defaults, the
Health data source), missing statements of defaults, returned errors,
cancellation, and destructive effects, and one convention sentence copied
into 36 files. Sixteen site pages carried the old admin verb names and six
samples no longer compiled.

## Decision

- CONVENTIONS.md ## Comments gains the exception to "no comment": every
  declaration reachable through `vulkan` states its contract -- a
  `Default:` line on each field WithDefaults fills, the Err* variables a
  verb returns, what cancellation does to a blocking verb and what runs
  beside it, what a destructive verb deletes and the
  ClientConfig.AllowDestroy gate. The path spelled is the caller's own,
  never the machinery verb behind it.
- A wrapper in `vulkan` states the facts a caller needs in its own words.
  It never points at another declaration for its contract.
- tools/conventions checks the `Default:` line over the alias closure.
  Machinery configs below the closure are exempt: their WithDefaults only
  backstops a direct caller.
- The inventory for such a review is the alias closure the conventions
  tests already compute, never a hand-kept list.
- Convention restatements are not comments: the Validate sentence is
  deleted everywhere, and nil-config clauses read `cfg may be nil or
  sparse.` / `options may be nil for the defaults.`
- Error-page fix and consequence lines mirror their Go declarations;
  three declarations changed so both name the client verb, and codes.json
  was regenerated.

## Consequences

Every reachable declaration's comment was checked against the code and
corrected where it disagreed. Three fix strings and one event consequence
changed text, which callers matching on message text would notice; the
codes are unchanged. Sixteen site pages and six error pages now spell the
client API. The review is closed; its ROADMAP item is removed.
