---
status: accepted
date: 2026-09-05
phase: pre-v1
---

# 0651 — documentation links related mechanisms in context

## Context

Board listings and adjacent-thread links make every document reachable, but
they do not tell a reader where a prerequisite or deeper mechanism becomes
relevant. A generic related-content box loses that context, while a link quota
rewards links whether or not they help. Generic anchor text also withholds the
destination from a reader scanning the sentence.

## Decision

When another thread owns a prerequisite, the detailed mechanism, or a relevant
contrast, documentation links its first useful mention. The anchor text names
what the reader will find. This is reviewed as prose: there is no link quota,
generated related-thread box, or mechanically added link.

## Consequences

An otherwise self-contained page owes no link. The page author decides whether
leaving the explanation inline would duplicate another page's job, then links
that page instead. Board membership remains the site's one structural grouping;
contextual links express only the relationship the surrounding sentence names.
