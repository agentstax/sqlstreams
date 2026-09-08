---
status: accepted
date: 2026-09-08
phase: pre-v1
---

# 0722 -- Repository support and build-output roots are hidden

## Context

Generated release artifacts, built executables, project records, developer
tooling, and the doc site occupied visible top-level directories beside the
library. They are repository support surfaces rather than packages a library
user starts from.

## Decision

The five roots are `.dist/`, `.bin/`, `.docs/`, `.tools/`, and `.website/`.
GoReleaser writes to `.dist/`. Locally built executables and generated schema
documentation live in `.bin/`. Records and working documents live in `.docs/`.
Repository-only tooling lives in `.tools/`, whose module paths are
`github.com/agentstax/vulkan/.tools` and
`github.com/agentstax/vulkan/.tools/compat`. The doc site lives in `.website/`.

The site's own Astro build output remains `.website/dist/`; it is internal to
the hidden site root. Schema-diagram generation writes its temporary `dist/`
inside `.bin/schema/`, never at the repository root.

## Consequences

This supersedes [0721] while preserving its directory boundaries. Current
commands, automation, links, and rule files use the hidden paths. Historical
records, HISTORY, THOUGHTS, and archive prose retain the paths they recorded.
