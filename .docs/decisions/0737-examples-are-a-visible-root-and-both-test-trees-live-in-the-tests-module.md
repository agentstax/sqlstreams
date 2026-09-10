---
status: accepted
date: 2026-09-09
phase: pre-v1
---

# 0737 -- Examples are a visible root and both test trees live in the .tests module

## Context

[0720] hid both dev-only roots as `.e2e/` and `.examples/`, and [0736]
added `.tests/` beside them for integration tests. That left three hidden
roots, the repo's tests split across two of them, and the examples -- the
first thing a visitor should find -- in a directory a listing does not
show.

## Decision

Runnable examples live at `examples/`, a visible root. Its module path,
`github.com/agentstax/sqlstreams/examples`, is unchanged.

`.tests/` is the one test module, `github.com/agentstax/sqlstreams/.tests`,
with two trees: `integration/` holds the datastore tests of [0736] and
the `postgres` seam; `e2e/` holds the programs of [0719] and their
`common` package. The e2e module merges into it: one go.mod, one
`go.work` line, one place `just verify` builds and vets.

## Consequences

Supersedes [0720]. [0719] stands, its path now read as `.tests/e2e/`.
The import `github.com/agentstax/sqlstreams/e2e/common` becomes
`github.com/agentstax/sqlstreams/.tests/e2e/common` in every e2e program
and in `.bench/claim`. Justfile recipes run `./.tests/e2e/<name>/main.go`;
`just test-integration` runs `./integration/...` inside `.tests`.
`go.work` drops `./.e2e` and uses `./examples`.
