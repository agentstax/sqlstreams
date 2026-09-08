---
status: accepted
date: 2026-09-05
phase: pre-v1
---

# 0655 — Code-page meta descriptions come from code facts

## Context

The 96 code pages deliberately carry no hand-authored description. Their meta
description therefore fell back to the title, repeating the problem, event,
metric, or alert name without explaining its classification or consequence.
Adding descriptions to every page would duplicate facts already rendered from
the page's checked declaration fields and create another value that can drift.

## Decision

The code-page route derives its meta description from `CodeThreadData`, the
same value used by the visible facts panel. The shape is
`Code <code> · <classification> — <consequence>` followed by
` · Fix: <fix>` only when the declaration carries a fix.

Code-page frontmatter continues to omit `description`; a collection test
enforces that boundary. Ordinary documentation pages keep their authored
description and existing title fallback.

## Consequences

Every error, event, metric, and alert page supplies search engines with its
operational meaning without a second maintained summary. A change to the
existing classification, consequence, or fix changes both the visible facts
and the meta description in the same build. Page titles and bodies are
unchanged.
