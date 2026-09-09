---
status: superseded by [0736]
date: 2026-09-09
phase: pre-v1
---

# 0731 -- vulkantest is one published fixture package with a schema per test

## Context

The ROADMAP carried `vulkantest` as a public helper (`NewClient(t)` against
a database it drops at cleanup) with isolation left open between a database
and a schema per test. Separately, the newest database test
(`claim_test.go` under the message consumer datastore) had prototyped its
own fixture: a fresh schema per test, `t.Cleanup` dropping it, and
hand-written DDL for five tables that will drift from the migration
registry.

Go's import rules decide the shape. An in-package `_test.go` cannot import
a package that imports its own package, and only the registration path may
create tables, which means `pkg/vulkan`. So a fixture that creates tables
can never be imported by an in-package test under `pkg/consume/...`; an
internal schema-only fixture below the controllers would leave every test
wiring its own controllers for tables, one shape per test. Two schemas of
one database no longer serialize on registration (every advisory lock key
carries the schema) and the pool sets no `search_path` [0632], so a schema
isolates what a test touches. Storj measured a schema per test at roughly
20ms against 140ms per database; River's `TestSchema` is the same choice for
tests that need cross-session locking, which transaction rollback hides.

## Decision

`pkg/vulkantest` is the one fixture package, published so a user's handler
tests use exactly what the library's own tests use. A fixture verb is the
constructor it stands for, with `t` in place of `ctx` and the fixture's pool
in place of the caller's: `NewDatastore(t, cfg)` (a datastore bound to a
fresh schema in the database `VULKAN_TEST_DATABASE_URL` names, dropped at
cleanup, a visible skip when unset), `NewClient(t, ds, cfg)` (a client over
`ds` plus system registration, so tables come from the registry),
`DatabaseURL(t)` (for a subject that takes a URL, the CLI), `WaitFor(t,
condition)` (the deadline poller), and `NewCountingLogger()` (log
assertions by level and code).

Isolation is a schema per test. A library test that needs the real table
set is an external test package (`package datastore_test`) built through
the fixture, reaching internals through `export_test.go`; no test writes
`CREATE TABLE` text. `VULKAN_TEST_DATABASE_URL` is the one env var name and
`vulkantest` the only reader. `.e2e/common` takes its pool from the same
variable and holds the one `must` / `die` / `assert` set.

## Consequences

The four existing variable names collapse into one. `claim_test.go` moves
to an external test package over `NewClient` and keeps its narratives. The
four exported verbs are supported surface and the alias-closure test treats
the package as reachable. CONVENTIONS Part 5's fixture section is the
spec; the site carries no page for the package until it ships. CI gains a
Postgres service in the change that lands the first test on the fixture.
The project rename [0725] carries the package name with it.
