---
status: accepted
date: 2026-09-09
phase: pre-v1
---

# 0736 -- Integration tests live in a nested .tests module over testcontainers

## Context

[0730] set three kinds -- pure, database, e2e -- with database tests
beside the code, skipping when `SQLSTREAMS_TEST_DATABASE_URL` was unset,
and [0731] published `pkg/sqlstreamstest` as the fixture they built on.
Working the first domain (worker) through that shape showed what it
cost: every database test needed a shared dev server and an env var the
`.env` never carried, the fixture was a published package users had no
demonstrated need for, and a test that wanted a server nobody else was
touching (the snapshot-xmax claim test) could not have one. The user's
own tests in that shape ran as one undivided sequence of statements.

testcontainers-go gives a test binary its own Postgres, but it is a
heavy dependency with a Docker client behind it, and the root module is
std lib plus pgx plus x/sync. Open-source Go libraries keep such
dependencies out of the library module one of three ways: accept them,
an env var plus an external server, or a nested test module (etcd's
`tests/`, grpc-go's `interop/`). The repo already runs `.e2e/` and
`.bench/` as nested dev-only modules.

## Decision

Two kinds beside e2e. A unit test lives beside the code, has no I/O and
no wait on the clock, and runs under `go test ./...` from the root with
nothing installed. An integration test lives under `.tests/`, a nested
dev-only module resolved through `go.work`, and runs against a Postgres
container its test binary starts through testcontainers, one schema per
test; `SQLSTREAMS_TEST_DATABASE_URL` names a server instead when set.

An integration test's subject is a domain's datastore driven through
its verbs. Controllers, assemblers, and the client get none. The tree
mirrors `pkg/<root>`, one directory per domain with its own
`setup_test.go`; `.tests/postgres` is the one Docker seam and the
variable's only reader.

Every test body is three captioned sections -- setup, test, verify.
Goroutines appear only where the named invariant is about concurrency.

## Consequences

Supersedes [0730] (its kinds and the beside-the-code database test) and
[0731] (the published fixture). `pkg/sqlstreamstest` is deleted once its
remaining readers under `.bench/` and the janitor tests move.
`claim_test.go` and the worker tests move under `.tests/`. CI runs
`.tests/` in a job with Docker and drops the Postgres service from the
root job. `just test-integration` is the recipe. CONVENTIONS Part 5
carries the rules.
