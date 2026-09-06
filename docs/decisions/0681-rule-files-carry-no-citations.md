---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# The rule files carry no decision citations; the decision map is the index

## Context

About a third of the rules in CONVENTIONS.md and AGENTS.md cited a
`[NNNN]` record inline; the rest did not. A reader could not tell
whether an uncited rule had no record or had simply never been
annotated, and every new record invited a hunt for the rule to hang it
on. docs/DECISION_MAP.md already indexes concept keywords to record
numbers and is loaded every session. AGENTS.md already says the rule
files hold the binding current rules and that today's rules are never
inferred by replaying decision history.

The same review reorganized CONVENTIONS.md into five parts, marked every
machine-checked rule `(checked)`, and stated each duplicated rule once.

## Decision

- CONVENTIONS.md and AGENTS.md state rules only. No `[NNNN]` inline.
- docs/DECISION_MAP.md is the one path from a rule to its why. A record
  that settles a rule adds its number to the map's line for that
  concept; it does not annotate the rule.
- HISTORY.md, ROADMAP.md, TODO.md, and record bodies keep citing records
  -- they are the narrative surfaces, and a citation there is the trail.
- A rule enforced by a `tools/conventions` test ends in `(checked)`; the
  marker is added in the same change as the test.

## Consequences

- A rule reads as a rule. The cost is one extra hop (grep the map) to
  reach a rationale; the map's keyword lines are that hop.
- CONVENTIONS.md's section names stay stable (website/CONVENTIONS.md and
  tools/conventions comments bind to them by name); reorganization moves
  sections under part headings, never renames them.
