---
status: superseded
date: 2026-09-08
phase: pre-v1
---

# 0716 -- Tools holds all repository-only developer tooling

## Context

The development database's Compose file, pgAdmin entrypoint, and schema
diagram configuration lived under `scripts/database`, while every command
that used them was developer tooling in the root Justfile. The existing
`tools/` rule described only dev-only Go modules, leaving non-Go tool assets
without a named home.

## Decision

`tools/` holds all repository-only developer tooling: programs, convention
checks, local service configuration, and scripts invoked by those tools. Go
tooling remains isolated in dev-only modules with its own dependencies;
non-Go assets live under the tool they support and need no module.

The database assets move together to `tools/database`, and the Justfile
remains their only command entry point. Production code may not import,
embed, or invoke anything under `tools/`.

## Consequences

`scripts/` no longer exists solely as a second developer-tooling root. A
new repository-only tool has one placement rule regardless of implementation
language, while Go dependency isolation and the root test boundary remain
unchanged. Record [0557]'s narrower module decision still governs Go tools.

Directory naming is superseded by [0721]; the single tooling-root rule remains
accepted there.
