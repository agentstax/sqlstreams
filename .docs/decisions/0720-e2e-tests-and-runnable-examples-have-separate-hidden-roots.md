---
status: superseded
date: 2026-09-08
phase: pre-v1
---

# 0720 -- End-to-end tests and runnable examples have separate hidden roots

## Context

Decision [0718] separated verification programs from runnable examples but
named the example root `.example/`. The singular name describes one example,
while the directory contains a collection of runnable examples.

## Decision

End-to-end tests and their support programs live under `.e2e/`. Runnable user
examples live under `.examples/`. Each root is a dev-only nested module,
excluded from the library's published zip and root test surface.

The nested module paths are `github.com/agentstax/vulkan/e2e` and
`github.com/agentstax/vulkan/examples`. The workspace and whole-repository
verification command include both modules independently.

## Consequences

This supersedes [0718]. Paths beginning `.example/` become `.examples/`.
Decision [0719]'s e2e terminology and recipe naming remain unchanged.

Superseded by [0737].
