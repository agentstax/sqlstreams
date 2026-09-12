---
status: superseded
date: 2026-09-12
phase: "pre-v1"
---

# Client package lives at the module root

## Context

The supported entry point lives at `pkg/sqlstreams`. The client should be
visible at the repository root without introducing another Go module.

## Decision

- Move `pkg/sqlstreams` to `client`, within the existing root Go module.
- Keep the package name `sqlstreams` and its exported API unchanged.
- Callers import `github.com/agentstax/sqlstreams/client`; maintained examples
  spell the import alias `sqlstreams` explicitly.
- Update current docs, imports, and convention scans together. Preserve
  historical records and benchmark evidence.

## Consequences

The client shares the root module's dependencies and release version.
Existing callers must update their import path; no forwarding package remains
at the old path. Convention checks include the new root.

The explicit-alias requirement is superseded by
[0762](0762-client-imports-use-the-declared-package-name.md).
