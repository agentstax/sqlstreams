---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# The doc site splits by page kind: a Reference board, one thread per handle

## Context

guides/client.mdx had grown to 7,300 words and 28 H2s, 40% of the site's
prose, and was four kinds of page in one file: a second quickstart, the
explanation of the API's shape, the only reference for every handle's
verbs, and changelog residue ("Client.Config has been removed"). The other
31 prose pages sat at a median of 900 words and 4-6 H2s.

Research on the doc sites developers name as the standard (PHP, React,
Supabase, Postgres, Stripe, Django, Tailwind) and the practitioner guides
(Diátaxis, Google, GitLab, Cloudflare, Astro, Canonical) converged: one
page per reference symbol with an identical skeleton and a guessable URL;
guides short, reference may run long; mechanical split triggers rather
than taste; sidebars two levels deep with one kind of thing per level;
and over-fragmentation (Pigweed, MAAS) as the opposite failure. The board
metaphor [0583] already gave codes and decision records [0596] a
one-thread-per-item board.

## Decision

- A Reference board: an index thread at /reference/, then one thread per
  handle or instance (client, pool, topic, producer, consumer, key,
  scheduler, system, manager, metrics, alerts) and per shared value type
  (message options, errors and events). Every thread reads the same way:
  the opening with the one example, `## Verbs` as a table, `## Config`
  with an H3 per struct and defaults from the declaration's `Default:`
  line, a subject-named section only for an owned mechanism, `## Gotchas`
  last. The skeleton is in website/CONVENTIONS.md ## Content.
- The explanation sections became concepts/api-shape. The first-program
  sections duplicated the quickstart and were dropped. Changelog prose
  and the old-verbs table were deleted, not moved.
- Boards are the Diátaxis split and each holds one kind of thread.
  handler-outcomes and consumer-group-config moved from Guides to
  Concepts; consumer-group-config lost its per-instance "version"
  sections to the reference threads. Roadmap stays on Getting Started:
  orientation is its kind. The nav gains a Reference link.
- Page-size triggers: a guide or concept thread splits past six H2s or
  roughly 1,500 words; a topic under three sentences folds in; reference
  may run long but never mixes kinds. A moved or split thread leaves an
  astro.config `redirects` entry at its old URL.

## Consequences

- Fourteen reference threads and one concept thread added, one thread
  deleted, two moved, three redirects.
- A reference thread is checked against the library's signatures and
  `Default:` lines; a change to a verb's contract or a field's default
  updates its row in the same change, the sibling of the error-page rule.
- client.mdx claimed a new group on `__system.alerts` starts at head;
  ConsumerConfig.Start defaults to `Beginning()`. The alerts thread says
  what the code does.
