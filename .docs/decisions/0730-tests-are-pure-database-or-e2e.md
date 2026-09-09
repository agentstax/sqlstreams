---
status: superseded by [0736]
date: 2026-09-09
phase: pre-v1
---

# 0730 -- Tests are pure, database, or e2e, and a database test runs against Postgres itself

## Context

The suite had grown three shapes with no rule behind them: 322 hand-rolled
`go test` functions, 60 `.e2e/` programs each declaring its own `must` /
`die` / `assert`, and `.docs/TEST.md`, 270 lines of Setup/Action/Assert
prose from a scratch harness. Tests touching Postgres hid behind four
different env var names and none ran in CI. [0328] deferred the datastore
interfaces' fate with "testing is the only remaining justification for the
interface"; the interfaces have since gone and nothing fakes the database.

The library's correctness lives in Postgres: `FOR UPDATE SKIP LOCKED`,
snapshot visibility, advisory locks, commit order. Split by unit versus
integration, nearly every test is "integration" and the label carries no
information. Google's footprint sizes (small, medium, large) and the
honeycomb shape database-backed services converge on fit: most tests are
one process against a real database, a thin layer is pure, and a few runs
are whole-system.

## Decision

Every test is exactly one kind, named by footprint: a pure test (one
process, no I/O, no wait on the clock), a database test (one process, a
real Postgres, a schema of its own), or an e2e test (a `.e2e/` program,
only for a second process, a signal, a killed backend, or a controlled
server). The lowest kind that can observe the behavior is the kind, and a
single-process scenario is never a new e2e program.

Postgres is never faked, mocked, or stood in for; the question [0328]
deferred closes with no fake datastore, ever. A test earns its place by
naming one of four reasons: a behavior at a public boundary, an invariant
SQL enforces, a regression, or a closed set. A test that restates the code,
guards a nil check, cannot name its invariant, or flakes is deleted. There
is no coverage target.

One toolkit repo-wide: `testing` from the standard library and one fixture
package [0731], got-before-want failure lines, `errors.Is` against `Err*`
variables, no assertion library, mock generator, clock library, container
library, or leak checker in any module.

## Consequences

CONVENTIONS gains Part 5, How code is tested; the E2E section moves into
it. `.docs/TEST.md` transcribes into database tests in `pkg/producer` and
`pkg/consumer` (single-process lifecycle cases) and pure tests in
`pkg/common` (SQLSTATE classification), with the signal and killed-server
cases staying e2e; the file is deleted when the last entry lands. Every
single-process `.e2e/` program converts to a database test before the
ROADMAP item closes. `just verify` and CI run database tests. The
research behind the rules is `TEST_EXPLORATION.md` at root until close-out.
