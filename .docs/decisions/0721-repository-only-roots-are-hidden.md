---
status: superseded
date: 2026-09-08
phase: pre-v1
---

# 0721 -- Repository-only roots are hidden

## Context

Generated release artifacts, project records, developer tooling, and the doc
site occupied four visible top-level directories beside the library. They are
repository support surfaces rather than packages a library user starts from.

## Decision

The four roots are `.dist/`, `.docs/`, `.tools/`, and `.website/`. GoReleaser
writes to `.dist/`. Records and working documents live in `.docs/`.
Repository-only tooling lives in `.tools/`, whose module paths are
`github.com/agentstax/vulkan/.tools` and
`github.com/agentstax/vulkan/.tools/compat`. The doc site lives in `.website/`.

The site's own Astro build output remains `.website/dist/`; it is internal to
the hidden site root. Schema-diagram generation writes its temporary `dist/`
inside `bin/schema/`, never at the repository root.

## Consequences

This supersedes the directory and module-path naming in [0557] and [0716]
while preserving their dependency-isolation and single-tooling-root rules.
Current commands, automation, links, and rule files use the hidden paths.
Historical records, HISTORY, THOUGHTS, and archive prose retain the paths they
recorded.

Superseded by [0722].
