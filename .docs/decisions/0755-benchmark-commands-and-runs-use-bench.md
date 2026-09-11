---
status: accepted
date: 2026-09-11
phase: "pre-v1"
---

# Benchmark commands and runs use bench

## Context

The shared runner lives at `.bench/` and runs all maintained benchmark
scenarios. Its former reliability name still appears in commands, binaries,
Compose projects, and current documentation.

## Decision

- Use `just bench <scenario>`, `just bench-smoke`, and `just bench-report`.
- Name the local and container binary `bench`, the default Compose project
  `bench`, and new run/project identifiers `bench_<timestamp>`.
- Update current documentation and diagnostics to these names; remove the
  former recipe names without aliases.
- Keep recorded evidence, historical ledger entries, and frozen-source
  reproduction commands under the names they originally used.
- This supersedes [0749](0749-benchmark-runner-lives-at-bench-root.md)'s command
  naming. Its root layout and evidence preservation continue unchanged.

## Consequences

Current commands describe the shared benchmark system. Existing results
remain readable because reporting does not depend on the run-name prefix.
Historical reproduction instructions explicitly identify their frozen source.
