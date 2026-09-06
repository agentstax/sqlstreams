---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# pkg/concurrency is infrastructure and lives under common

## Context

CONVENTIONS ## Package layout says every package is exactly one of three
kinds: infrastructure (`common` and its subpackages, plus `datastore`),
a domain (a `pkg/<root>` with its controller, datastore, and workers), or
an API package. `pkg/concurrency` -- Permit, Pool, Queue, imported by the
consume stack and the consumer assembler -- was none of them by position.
It has no Vulkan noun, no SQL, and no codes, which is the infrastructure
definition, yet it sat beside `topic` and `produce` at the top of pkg/,
where a reader expects a domain root or an assembler.

The rule-file review of 2026-09-06 surfaced it as the one package the
three-kinds rule could not place.

## Decision

- `pkg/concurrency` moves to `pkg/common/concurrency`, beside
  `common/diagnostic` and `common/logging`. The package name and its API
  are unchanged; four import lines moved.
- The infrastructure bullet names it. The rule keeps its "exactly one of
  three kinds" wording with no exception clause.

The alternative -- list it as a fourth top-level infrastructure package
-- was rejected: it costs one exception in the rule and one directory in
pkg/ that reads as a domain and is not.

## Consequences

- A generic primitive with no domain owner is placed under `common`, not
  at pkg/ top level. The directory listing of pkg/ is domains,
  assemblers, `common`, `datastore`, and `vulkan`.
- Checked by build, vet, gofmt, and `go test -race` on the moved package
  and its importers; no behavior moved.
