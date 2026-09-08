---
status: accepted
date: 2026-09-08
phase: pre-v1
---

# 0723 -- Repository support, build-output, and benchmark roots are hidden

## Context

Generated artifacts, project records, developer tooling, the doc site, and
benchmark harnesses occupied visible top-level directories beside the library.
They are repository support surfaces rather than packages a library user starts
from.

## Decision

The six roots are `.dist/`, `.bin/`, `.docs/`, `.tools/`, `.website/`, and
`.bench/`. The benchmark module path is `github.com/agentstax/vulkan/.bench`.
The workspace, CI, recipes, Docker contexts, code imports, and current
documentation use the hidden path.

The site's Astro build output remains `.website/dist/`. Schema-diagram
generation writes inside `.bin/schema/`.

## Consequences

This supersedes [0722] and the benchmark directory naming in [0541] while
preserving their directory and dependency boundaries. Historical decisions,
HISTORY, THOUGHTS, archive prose, and raw benchmark run logs retain the paths
they recorded.
