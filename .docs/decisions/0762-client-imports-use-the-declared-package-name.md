---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# Client imports use the declared package name

## Context

The client should stay in its own directory without a repeated product name
in the import path or a generic package name that competes with variables.
[0760](0760-client-package-lives-at-the-module-root.md) required explicit
aliases even though Go already uses the declared package name.

## Decision

- Keep `client/` in the root module, declaring `package sqlstreams`.
- Import `github.com/agentstax/sqlstreams/client` without an explicit alias
  in maintained code and examples. Callers continue to use `sqlstreams`.
- Convention import scans recognize this directory/package-name difference.
- This supersedes [0760](0760-client-package-lives-at-the-module-root.md)'s
  explicit-alias requirement; its package placement remains unchanged.

## Consequences

The import path names the product once, client files stay out of the
repository root, and callers keep `client` available as a variable name.
The directory and package names differ; no API or module boundary changes.
