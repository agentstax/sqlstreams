# Testing: exploration

Current names follow the SQLStreams rename; section 1 retains the original
inventory names as observed before the rename.

2026-09-09. Research and the rule sheet behind the ROADMAP item "Test
suite: kinds, sqlstreamstest, TEST.md transcription, e2e conversion". The
rules landed the same day as CONVENTIONS Part 5, an AGENTS Verification
edit, records [0730] and [0731], and the proposed page
`/reference/sqlstreamstest/`. This file keeps the research, the invariant
table, and the entry-by-entry map of `.docs/TEST.md` (section 5) until
that work closes, then is deleted. Nothing in it is built.

Three questions the item raised, answered in one line each:

- **When is a test worth writing?** When it pins a behavior a caller or
  operator could observe, and would still pass after any rewrite that
  keeps that behavior (section 3.2).
- **Which kind of test?** The lowest footprint that can observe the
  behavior: pure, then database, then e2e (section 3.3).
- **What toolkit?** One: `testing` plus a single fixture package.
  No assertion library, no mocks, no fake datastore, no clock library
  (section 3.4).

## 1. What the repo has today

Facts from an inventory of every module, 2026-09-09.

- 81 `*_test.go` files, ~322 test functions, root module `go test
  ./...` in 7s (17s with `-race`). Zero third-party test dependencies
  anywhere. Assertions are hand-rolled `if got != want { t.Fatalf }`.
- Tests that touch Postgres are gated by **four different env var
  names** (`VULKAN_TEST_DSN`, `VULKAN_CLI_TEST_DATABASE_URL`,
  `VULKAN_WORKER_TEST_DATABASE_URL`, `RELIABILITY_TEST_DSN`), each with a
  `t.Skip` when unset. `just verify` and CI set none of them, so **no
  database test has ever run in CI**. `verify` also runs no tests in
  `cmd/vulkan` or `otel` (build and vet only).
- `.e2e/` holds 60 `package main` programs, 50 with a `*-e2e` recipe.
  Each re-declares `must` / `die` / `assert`, panics a private failure
  value, recovers it in `run`, and exits 1. Postgres is hardcoded to
  `example_db` on `:5432`. Two of 60 assert on logs through a counting
  logger, the shape CONVENTIONS mandates.
- `.bench/reliability` is the ledger-and-checker harness [0687, 0696,
  0711]: producers and consumers write records, the checker joins them
  in SQL and judges six declared safety checks. It is the repo's only
  whole-system correctness test and the only fault-injection surface.
- `.tools/conventions` runs the `(checked)` rules as tests, source as
  data, with the sabotage rule (feed it wrong input before trusting
  green).
- The newest test, `pkg/consume/messageconsumer/controller/datastore/
  claim_test.go` (uncommitted), is a distinct new idiom and a good one:
  a fresh schema per test, `t.Cleanup` drops it, `t.Context()` in the
  body, the real datastore constructor chain, MVCC narratives that
  cannot be tested any other way. Its one flaw: it hand-writes DDL for
  five tables, which will drift from the migration registry.
- No `t.Parallel()`, no `export_test.go`, no shared fixture package,
  no counting logger inside `pkg/`, no fake datastore. Decision [0328]
  deferred the datastore interfaces' fate with "testing is the only
  remaining justification for the interface, and the current shape does
  not clearly serve even that".
- `.docs/TEST.md` is 270 lines of Setup/Action/Assert prose: 14
  producer and 18 consumer lifecycle scenarios, plus a retry
  classification catalogue graded by how each SQLSTATE can be
  triggered. Two entries are already marked "(FIXED, was flaky)" with
  N=6 / N=8 repeat runs recorded by hand.
- Existing written rules: CONTRIBUTING "changed behavior has a test;
  tests call the real datastore methods, never a copy of their SQL";
  CONVENTIONS "e2e tests assert on log events by level and attributes
  through a counting Logger, never by matching message substrings";
  AGENTS Verification (targeted `go test -race` per change, full
  fresh-DB e2e suite at review-ready checkpoints only).

## 2. The frame

### 2.1 What kind of system this is

The library's correctness lives in Postgres: `FOR UPDATE SKIP LOCKED`,
snapshot visibility, advisory locks, transaction commit order. Almost
no domain logic runs without a database, so a test suite split by the
classic unit / integration labels puts nearly everything in
"integration" and the label stops carrying information.

Google's *Software Engineering at Google* (ch. 11) classifies tests by
**footprint** instead: small (one process, no I/O, no sleep), medium
(one machine, may use localhost network and a database), large
(several machines or processes). Scope is a separate axis. The
Spotify "honeycomb" and the testing-trophy school say the same thing
for database-shaped services: the bulk of the suite is one process
against a real database, a thin layer of pure tests for isolated
logic, and a few whole-system runs. That is the shape this library
needs, and it is already the shape the repo has by accident (the e2e
programs are mostly single-process database tests wearing a `main`).

Spelled in the repo's own nouns rather than Google's sizes:

| Kind | Footprint | Where | Runs in |
| --- | --- | --- | --- |
| **pure test** | one process, no I/O, no sleep, no goroutine wait on the clock | `_test.go` beside the code | `go test ./...` always |
| **database test** | one process plus a real Postgres it owns a schema in | `_test.go` beside the code | `go test ./...` when `SQLSTREAMS_TEST_DATABASE_URL` is set, else a visible SKIP |
| **e2e test** | more than one process, a signal, a killed backend, or a controlled Postgres | `.e2e/<name>/main.go`, `just <name>-e2e` [0719] | review-ready checkpoints |
| **reliability lab** | the ledger-and-checker harness under load and faults | `.bench/reliability`, `just reliability-lab` | release checkpoints [0711] |

The `.tools/conventions` tests are a fifth thing (tests of the rule
sheet, not of the library) and keep their existing rules.

### 2.2 The invariants the product makes

Every serious queue tests the same short list, under the same names
(Jepsen's queue checker, Kafka's ducktape validator, WarpStream's three
assertions, RabbitMQ's quorum-queue property suite). Written in this
repo's nouns, these are the behaviors the suite exists to pin. A test
that cannot name which one it protects is the first candidate for
deletion.

| Invariant | What it means here | Home |
| --- | --- | --- |
| no loss | every produced message id is delivered at least once before retention removes it | reliability lab check; database tests per verb |
| bounded duplication | a message is redelivered only after its lease expired or its handler returned an error | reliability lab check; database tests on reclaim |
| one live lease | no two consumer instances hold a live claim over the same message | database test (two sessions, one process) |
| monotonic cursor | `consumer_group_cursor` never regresses [0392] | database test |
| snapshot fence | a claim never reads past a message whose producer transaction is still open | database test (`claim_test.go` already does this) |
| per-key order | ordered and exclusive keys deliver in id order, one at a time | database test; reliability lab check |
| idempotent produce | one idempotency key yields one row under concurrent callers | e2e today (`idempotencykeysrace`), a database test tomorrow |
| crash consistency | SIGKILL of a consumer instance loses nothing; committed work stays, in-flight work is redelivered once | e2e (`crash`, `shutdowntruncation`) |
| lifecycle contract | Register/Produce/Consume/cancel behave as `.docs/TEST.md` records | database tests in `pkg/producer`, `pkg/consumer` |
| retry classification | each SQLSTATE the retry wrapper names retries or does not | pure test, one table |

## 3. Proposed rules

Written as CONVENTIONS-style rule lines, full shape, for trimming.

### 3.1 Kinds and placement

- Every test is exactly one of pure, database, or e2e (table in 2.1).
  A pure test never sleeps, never opens a connection, never waits on
  the clock. A database test owns a schema it creates and drops. An
  e2e test is a `.e2e/` program and exists only because a database
  test cannot observe the behavior (a second process, a signal, a
  killed backend, a controlled Postgres).
- Pure and database tests live in `_test.go` files beside the code,
  in the same package by default. A test that needs the real table set
  is written as an external test package (`package datastore_test`)
  and builds the tables through the fixture package, never through
  hand-written DDL; `export_test.go` exposes what the external package
  must reach.
- CONTRIBUTING's rule extends to DDL: tests create tables only through
  the real registration verbs (the migration registry), never a copy of
  the `CREATE TABLE` text.
- Tests of the rule sheet stay in `.tools/conventions` under its
  existing rules (source as data, `-count=1`, sabotage before trust).

### 3.2 When a test earns its place

A test earns its place by one of four reasons, and names it in the
test name or its first comment:

- **behavior** -- a caller or operator could observe the outcome at a
  public boundary (a `sqlstreams` handle verb, a controller verb, a log
  line's level and code, a CLI exit). The Beyoncé rule: a behavior the
  project relies on has a test or may be broken by anyone.
- **invariant** -- one of the rows in 2.2, or a SQL fact a rewrite could
  silently lose (the snapshot fence, `GREATEST` on the cursor).
- **regression** -- it fails before the fix and passes after; the
  reproducer of a bug is the test.
- **closed set** -- a table over every legal value (SQLSTATEs, error
  rendering per surface, cron expressions) so a new value cannot be
  forgotten.

A test is deleted, not fixed, when it:

- **restates the code** -- passes or fails with the call sequence or
  SQL text rather than the outcome (Google's change-detector test);
  asserting "method X was called", matching an error's message text,
  or a full-struct compare of an internal row that changes on every
  column add are the tells.
- **guards a constructor nil check or a trivial Validate branch** --
  those are review-enforced by the constructor rules. A Validate test
  exists only where constraint math exists (the RetryPolicy overflow
  table is the model).
- **cannot say which invariant it protects** -- the answer to "is an
  integration test you don't understand valuable": no. Name the
  invariant or do not write the test.
- **is flaky** -- it is quarantined the day it flakes (skipped with the
  issue in the skip text) and either made deterministic or deleted
  within the same milestone. Google's data: a suite near 1% flake rate
  loses trust.

No coverage target, ever. Coverage says which lines ran, not whether
anything was asserted. At review-ready checkpoints, a mutation run on
touched packages (gremlins, dev-only) is the honest score and the
antidote to green tests with no assertions; it is a checkpoint tool,
never a per-change gate.

### 3.3 Which kind

- The lowest footprint that can observe the behavior. Pure when the
  logic is a function of its arguments. Database when SQL runs. E2E
  only for a second process, a signal, a killed backend or server.
- Never a fake, mock, or in-memory stand-in for Postgres. The behavior
  under test IS the lock manager and MVCC; a fake would pass tests the
  real database fails. [0328]'s deferred question closes here: the
  datastore interfaces have no testing justification and go.
- A behavior a higher kind finds with no lower kind failing gets the
  lower test written (Fowler); a higher test duplicating a lower one is
  deleted.
- `testing/synctest` is for pure tests of in-process timing only (the
  batcher's flush timer, the suppression window, retry curves,
  shutdown grace). A goroutine blocked on Postgres is never durably
  blocked, so a bubble cannot host a database call; structure loops so
  the body is a synchronous function a test calls directly.

### 3.4 One toolkit

- `testing` from the standard library, nothing else, in every module.
  Not testify, not go-cmp, not gomock, not testcontainers, not goleak,
  not a clock library. The main module's dependency rule forbids them;
  the nested modules could take them and do not, so one style holds
  repo-wide.
- Failure lines read got-before-want and name the verb and input:
  `Verb(%v) = %v, want %v`. Structs compare with `reflect.DeepEqual`
  and print with `%+v`. Errors branch with `errors.Is` against the
  `Err*` variable, never message text (the wording stays free to
  improve, the same rule as production code).
- `t.Fatal` for setup and for any step later steps depend on;
  `t.Error` for independent checks inside one case. Never `t.Fatal`
  from a goroutine the test spawned: send to a channel the test reads.
- Table-driven with `t.Run` and a name per case (never an index) for
  closed sets. A multi-step database narrative (claim, hold a
  transaction, claim again) is a plain sequential test, no table.
- Test names are sentences in the repo's nouns:
  `TestEmptyClaimPersistsPendingObservation`, not `TestClaim2`.
- Helpers take `testing.TB`, call `t.Helper()`, register teardown with
  `t.Cleanup` (never a returned closure), and fail with `t.Fatal` only
  for setup. Assertion helpers are not written: the failure line
  belongs in the test function.
- A test that needs to wait for work never sleeps for it. It either
  reads a channel the handler closes, or polls with a deadline through
  the fixture's `WaitFor`, which fails with the last error when the
  deadline passes (never hangs). The deadline is 3s locally and 10s
  under CI.

### 3.5 The fixture package: `pkg/sqlstreamstest`

The ROADMAP's `sqlstreamstest` item and the database-test fixture are one
package. It is published (the `net/http/httptest` precedent) so a user
can test their handlers with the same three verbs the library's own
tests use. One import, one style.

- `NewDatastore(t testing.TB) *datastore.PostgresDatastore` -- reads
  `SQLSTREAMS_TEST_DATABASE_URL`, skips with a visible message when unset,
  opens one lazily built pool per package with a capped `MaxConns`,
  creates `test_<package>_<n>` as the schema, returns a datastore
  bound to it, and drops the schema in `t.Cleanup` under
  `context.WithoutCancel` (the test's own context is already
  cancelled by then). Schema-per-test, not transaction-per-test:
  rollback isolation hides exactly the cross-session locking a queue
  is about. Storj measured a schema at ~20ms against ~140ms per
  database and 1.6s+ per container.
- `NewClient(t testing.TB) *sqlstreams.Client` -- `NewDatastore` plus
  `RegisterSystem` through the public client, so tables come from the
  registry. This is the ROADMAP's 50-line estimate.
- `WaitFor(t testing.TB, condition func() error)` -- the deadline
  poller of 3.4.
- `NewCountingLogger() *CountingLogger` -- the `logging.Logger` that
  counts by level and code, replacing the two hand-rolled copies in
  `.e2e/`, so log assertions are one shape everywhere.

Rules the package carries: it holds test verbs only, declares no codes,
owns no SQL beyond `CREATE SCHEMA` / `DROP SCHEMA`, and is the one
package allowed to read a `SQLSTREAMS_TEST_*` env var. The alias-closure
test treats it as reachable surface.

### 3.6 Time

- No clock seam and no clock library. Intervals are already config
  (poll rates, lease durations, grace windows); a database test sets
  them short (10 to 100ms) and waits through `WaitFor`. Lease-expiry
  tests may set `expires_at` back in time through the real datastore
  verb where one exists, never raw SQL on the row.
- Pure timing logic is tested under `synctest` with the fake clock.
- If a real seam is ever needed, River's `TimeGenerator` (`Now()` for
  reads, `NowOrNil()` so writes use `NOW()` in production) is the
  known shape; not before a test demands it.

### 3.7 Running and CI

- `go test ./...` with no env var runs every pure test and skips every
  database test visibly. With `SQLSTREAMS_TEST_DATABASE_URL` set it runs
  both. One env var name repo-wide; the four current names collapse
  into it.
- `just verify` runs `go test -race -count=1 -shuffle=on ./...` in
  every module that has tests (adding `cmd/sqlstreams` and `otel`), against
  the dev Postgres, and prints the shuffle seed. Per-change loops keep
  the test cache (drop `-count=1`) because a cached green is what makes
  an agent loop cheap.
- CI adds a Postgres service and exports the env var; a database test
  that cannot run in CI is dead code.
- `-race` is on everywhere until a test's memory makes it impossible;
  then that test alone moves to a no-race lane by name (pgx and NATS
  both keep one).
- No `t.Parallel()` until the database suite exceeds a minute; with
  schema-per-test it is one line to add later, and its absence keeps
  `t.Setenv` legal and the pool small.
- Goroutine leak assertions do not appear in pure or database tests
  (`runtime.NumGoroutine` baselines are flaky by construction); the
  abandoned-goroutine mechanisms are covered by e2e tests that observe
  the outcome, not the count.

### 3.8 E2E programs

- Keep [0719]: one program per scenario, `run() error`, the recovered
  failure value, exit 1. Replace the 60 private copies of
  `must` / `die` / `assert` with one `.e2e/common` set, and take the
  pool from the fixture package (env var, not a hardcoded `example_db`).
- Every new single-process scenario is a database test, not an e2e
  program. Existing programs migrate when touched; the ones that need
  a second process, a signal, or a killed server stay.
- Signals are tested by running the library as a child process and
  sending SIGTERM/SIGKILL, then asserting database state (lease
  reclaimed, message delivered once more) -- what `crash` and
  `shutdowntruncation` do today. A `TestMain` re-exec of the test
  binary is the way to do the same inside `go test` if a signal case
  ever needs to leave `.e2e/`.

### 3.9 The reliability lab

Unchanged and load-bearing: it is the only test whose assertions are
the whole-system invariants of 2.2 under real faults, and every green
run is trusted only after sabotage [0687]. The fault list it should
grow toward, in order of cost: `pg_terminate_backend` on a claim
connection, `statement_timeout` / `lock_timeout` set low, SIGKILL of a
consumer process, toxiproxy `timeout` / `reset_peer` between pool and
server, `synchronous_commit=off` plus a postmaster kill. Toxiproxy and
container control live here and never in the main module.

## 4. Agent-era answers to the THOUGHTS questions

- **Start with unit tests or wait for the API to settle?** Write
  behavior and invariant tests now at the boundaries that are already
  stable (datastore verbs' SQL invariants, the lifecycle contract),
  because those survive renames; write nothing that pins structure
  until the API freezes. The one controlled comparison of TDD-in-the-
  loop (Böckeler, martinfowler.com, 2026) found no quality difference
  at 3 to 8x token cost, so the rule is spec first, test the behavior,
  never test-first as ritual.
- **What is a valuable test when an agent writes the code?** One the
  agent can run as its stop condition, that fails for one reason, and
  that constrains the rewrite (mutation score, not coverage). Beck,
  Anthropic's own guidance, and Willison converge on the same three
  operational rules: commit the failing test before the fix; a test is
  never edited to pass in the same change that touches the code it
  covers without the diff saying so; the agent proposes tests, the
  reviewer keeps the ones that name an invariant.
- **Is testing validation logic valuable?** Only where constraint math
  exists. A nil check or a `must be positive` guard is enforced by the
  constructor rules and review; testing it is a change detector.
- **Integration tests you don't understand?** Not valuable, and the
  fix is the naming rule in 3.2: a test states the invariant it pins
  in its name. If that sentence cannot be written, the test is not
  written.

## 5. Migration notes: today's code onto the rules

### 5.1 `.docs/TEST.md`, entry by entry

| Entry | Kind | Home | Note |
| --- | --- | --- | --- |
| P1 -- P12 | database | `pkg/producer` | "sleep briefly" steps become `WaitFor` or a handler channel |
| P13 SIGKILL, P14 SIGTERM | e2e | `.e2e/crash`, `.e2e/shutdowntruncation` | already the shape; give `crash` a recipe |
| MISC batcher goroutine baseline | dropped | -- | `NumGoroutine` assertion; the idle-exit behavior is observed by P1/P5 probes |
| C1 -- C14, C16 | database | `pkg/consumer` | C2, C10, C11 use a handler channel, not a flag plus sleep |
| C15, C15b | e2e | `.e2e/shutdowntruncation` | double-signal escalation is a subprocess test |
| C17 backend killed | database | `pkg/consumer` | `pg_terminate_backend` from the test's own second connection, one process (pgx's own suite does this) |
| MISC N=6 / N=8 runs | dropped | -- | history, keep in HISTORY only |
| IsTransientPgError tables | pure | `pkg/common` | `retry_datastore_test.go` is the model; extend its table to all 32 codes |
| RETRY-40P01 | e2e | `.e2e/compactiondeadlock` | exists |
| RETRY-53300, 57014 | database | `pkg/common` or `pkg/consumer` | `MaxConns: 1` and `pg_cancel_backend` are both one-process |
| RETRY-57P05 | dropped | -- | TEST.md already grades it fiddly and lowest priority |
| RETRY-08000/08006/08001/08003, 57P02, 57P03 | pure now, lab later | `pkg/common`; reliability lab | synthetic `*pgconn.PgError` through the real `Wrap` proves the mechanism; toxiproxy and a controlled server are ROADMAP "chaos-testing surfaces" |
| RETRY-40001, 08007, 40003 | pure | `pkg/common` | unreachable by design; synthetic only, and that is fine |
| MISC dropPartition retry | database | `pkg/topic/janitor` | two `DROP TABLE IF EXISTS` calls |

### 5.2 Code and tooling

- Collapse the four env var names into `SQLSTREAMS_TEST_DATABASE_URL`
  (four files plus `.bench`).
- `claim_test.go`: replace the hand-written DDL with
  `sqlstreamstest.NewClient` and an external test package; keep every
  narrative unchanged. It is otherwise the model database test.
- Add `pkg/sqlstreamstest` (doc page first, per the ROADMAP item; the
  page is the spec).
- `.e2e/common`: add `Must`, `Die`, `Assert`, and the pool from the
  env var; delete the 60 copies as programs are touched.
- `just verify`: `-count=1 -shuffle=on`, run `cmd/sqlstreams` and `otel`
  tests, require the env var. CI: Postgres service.
- `.docs/TEST.md` is deleted when 5.1 lands; its retry catalogue's
  reasoning moves into one comment per synthetic case.
- Record-keeping: a decision record for the three kinds and the
  no-fake rule (closing [0328]), one for the fixture package and
  schema-per-test isolation (the ROADMAP item names per-database vs
  per-schema as open; `claim_test.go` already picked schema), and the
  `## Tests` part in CONVENTIONS.md with `(checked)` candidates: no
  third-party test import, no `time.Sleep` in a `_test.go` outside
  `synctest`, one env var name, no hand-written `CREATE TABLE` in tests.

## 6. Rejected, with reasons

- **testify / go-cmp / gomock**: forbidden in the main module by the
  dependency rule; allowing them in nested modules alone would make two
  styles. Google's style guide discourages assertion libraries on
  failure-message grounds anyway.
- **A fake or in-memory datastore, pgx mocks**: the behavior under test
  is Postgres. River, pgx, gue and NATS all test against the real
  server.
- **Transaction-rollback isolation**: fastest, and blind to
  `SKIP LOCKED`, advisory locks, and commit visibility. River itself
  documents `TestSchema` for exactly the cases that need cross-session
  behavior.
- **testcontainers / a container per test**: 1.6 to 2.9s per container,
  Docker-daemon and reaper fragility in CI. One long-lived Postgres per
  `go test` run, named by env var, is what River, pgx and gue do.
- **A clock library**: benbjohnson/clock is archived; clockwork is a
  dependency with no seam to hang on. Short real intervals plus
  `synctest` cover the cases today.
- **goleak**: incompatible with `t.Parallel`, needs pgxpool ignores,
  and the leaks that matter are observed as behavior in e2e tests.
- **Deterministic simulation testing**: every in-process Go DST
  (gosim, Polar Signals' WASM build, Resonate) excludes an external
  Postgres by construction. Antithesis is the only form that keeps a
  real Postgres in the loop (WarpStream and Aiven ran it) and is a paid
  trial at a release checkpoint at most, never before.
- **Property-based testing as a standard tool**: rapid would be a
  dependency and the pure surface is small (cron next-run, backoff,
  name length). Revisit if a state-machine model of claim/lease rules
  ever stays sub-second; not a v1 tool.
- **Coverage targets and TDD-as-ritual**: section 3.2 and 4.
- **`-short` and build tags for database tests**: a build tag hides the
  tests from `go test ./...`; the env-var skip keeps them visible as
  SKIP lines (pgx's own advice).

## 7. Forks, settled 2026-09-09

- **A. `pkg/sqlstreamstest` published.** An in-package `_test.go` cannot
  import a fixture that imports `pkg/sqlstreams` (import cycle), and only
  the registration path may create tables. So the fixture is one
  published package, and library tests that need real tables are
  external test packages (`package datastore_test`) reaching internals
  through `export_test.go`. Two packages (an internal schema fixture
  plus a public helper) would leave every test wiring its own
  controllers for tables, which is the inconsistency to avoid.
- **B. Existing e2e programs migrate opportunistically, and every
  single-process one is converted before this item closes.** The
  review-ready checkpoint runs whatever is left as e2e until then.
- **C. CI Postgres lands after the fixture**, in the change that
  brings the first database test onto it.

## 8. Consistency is the rule, not a preference

One shape per kind, and the sameness is enforced rather than reviewed:

- Every database test opens with `sqlstreamstest.NewClient(t)` (or
  `NewDatastore(t)` when no tables are needed); every wait goes through
  `WaitFor`; every log assertion through the counting logger; every
  e2e program through `.e2e/common`.
- `(checked)` candidates for `.tools/conventions`: no import outside
  the standard library and this repo in a `_test.go`; no `time.Sleep`
  in a `_test.go` outside a `synctest` bubble; no `CREATE TABLE` text
  in a `_test.go`; exactly one `SQLSTREAMS_TEST_*` env var name, read only
  by `sqlstreamstest`; no e2e program declaring its own `must` / `die` /
  `assert`.
- The doc page for `sqlstreamstest` shows the one pattern, and the
  library's own tests are its examples.

## 9. Session state, 2026-09-09 evening (read this first when resuming)

Everything before this section is research and the first attempt. The
first attempt was thrown out: the user judged the test code unstructured,
audited every `_test.go` under pkg/ and otel/, and kept 17 files (listed
below); the other 41 were deleted with `git rm` and the survivors were
rewritten. Then the approach changed again to what is now the rule.

### The rule now (CONVENTIONS Part 5, record 0736)

- Unit tests beside the code: no I/O, no clock. `go test ./...` from root.
- Integration tests under `.tests/`, a nested dev-only module over
  testcontainers (postgres:18, one container per test binary, one schema
  per test). `SQLSTREAMS_TEST_DATABASE_URL` replaces the container when set.
  `just test-integration`; `just verify` includes it.
- The subject of an integration test is a domain's datastore driven through
  its verbs. Controllers, assemblers, and the client get none (user's call).
- Every test body: `// setup`, `// test`, `// verify` sections. Goroutines
  only where the named invariant is concurrency. Expiry by UPDATE, never a
  wait. Setup through real registration verbs.
- One directory per domain root (`.tests/integration/worker`, `.tests/integration/consume`), each
  with `setup_test.go` holding that domain's helpers; `.tests/integration/postgres` is
  the only Docker seam and the only reader of the env var.

### What exists

- `.tests/go.mod` (no require for the root module -- `go mod tidy` writes a
  pseudo-version line every time; strip it), `.tests/integration/postgres/postgres.go`
  (`Start(t) *datastore.PostgresDatastore`), `.tests/integration/worker/{setup,instance}_test.go`,
  `.tests/integration/consume/{setup,claim}_test.go` (claim_test moved from pkg via git mv).
- Records: 0736 accepted; 0730 and 0731 flipped to superseded. Ledger, map,
  ROADMAP item (rewritten), AGENTS verification, justfile, CI updated.
- go.work has `./.tests` (go.work is gitignored; CI's `go work init` line
  has it too).
- Surviving root unit tests, all rewritten to Part 5 shape and committed by
  the other session: retry_datastore (synctest), retry_policy, diagnostic
  error + placeholder, schedule expression, robfig (vendored, untouched),
  otel validation, alert history, partitioncount evaluate, pool DSN,
  migrate Validate, both registry tests.

### The worker promise list (approved; all 12 written 2026-09-09)

#1 ran green; #2-#12 were written under a benchmark hold (no Docker) and
have only compiled and vetted. First step on resume:
`cd .tests && go test -race -count=1 -v ./integration/worker/`. Watch #10's
metadata DeepEqual (jsonb decodes to map[string]any) and #8's clock
comparison. Files: instance_test.go (1-7), instance_log_test.go (8-9),
worker_test.go (10-12), setup_test.go (helpers: newWorkerDatastore,
declareWorker, claimInstance, expireInstance, rejectInstanceLogInserts,
declareStreamOwner, declareConsumerGroupOwner).

1. concurrent claims at target 1 yield one instance -- DONE, ran green
   `.tests/integration/worker/instance_test.go` (errgroup, 16 claimants)
2. claim: target 0 declines, -1 always claims, missing row declines, expired
   instance no longer counts
3. renew/record/release return ErrInstanceLost for wrong token, expired,
   released, and write nothing
4. release then claim at target 1 succeeds at once
5. claim and renew cannot commit without their worker_instance_log row
   (CHECK (false) NOT VALID on the log table)
6. failure count increments and returns; success resets
7. SweepExpiredInstances removes only expired rows, returns the count
8. GetInstanceHistory window: expired-before-window out, live in, renewal is
   its own snapshot (ListInstanceSnapshots at the datastore)
9. SweepExpiredInstanceLogs retains by lease expiry plus ttl, not record age
10. RegisterWorker: redeclare writes metadata + one log row; unchanged writes
    no log row; target_instances kept (suspension survives restart)
11. two concurrent first declarations both nil, one row
12. ListWorkers by owner chain UPWARD (what a manager on the owner runs):
    group sees own + stream's + system's, stream sees own + system's, system
    sees all, never a sibling's or a child's; owners resolve from join columns
No tests for guards, GetWorker, DeclareWorker's gate, RegisterInstance,
AssertSchemaSupported, ErrWorkerDeclarationInterrupted.

### The consume promise list (approved 2026-09-09; all 18 written 2026-09-10)

All 18 ran green 2026-09-10 (22 tests with the three claim-fence tests
and one extra split), after one setup fix in committed_test.go.
One test file per datastore under `.tests/integration/consume/`; the three
claim-fence tests in claim_test.go stay (rewritten to the controller-based
setup). deliveryconsumer is archived: no tests. Files: commit_test.go
(1-3), exception_test.go (4-10), key_lease_test.go (11-13), group_test.go
(14-16), committed_test.go (17), binding_log_test.go (18). setup_test.go:
newMessageConsumerDatastore(t) (groups, consumer) plus one-call datastore
builders (exception, metric, cursor advancer, key lease, consume, janitor),
produceMessages, produceKeyedMessages, insertException,
insertKeyedException, claimRange, registerConsumer, declareLiveInstance,
holdTransaction. Verify goes through library verbs where one exists
(metric snapshot, exception Claim, AdvanceCommitted, ListGroupBindingConfigLog);
direct SQL only for delivery_log, binding_config, settled_head.

messageconsumer (commit_test.go)
1. Commit with a stale token -> ErrLeaseLost, no exception rows written
2. Commit writes one exception row per outcome kind (status, delays,
   can_run_after), none for success/superseded, log row at attempt 0 --
   values table
3. PartialCommit narrows low to lastProcessed; token, high, expiry kept
exceptionconsumer (exception_test.go)
4. Claim eligibility by status: ready past can_run_after, inflight past
   lease expiry, deferred always; ready-early, live inflight, done, dead,
   superseded never -- values table
5. Claim skips a key under an unexpired key lease, and an ordered row with
   an earlier unresolved same-key row
6. Claiming an expired inflight row writes one 'expired' delivery_log row
   at the pre-claim attempt
7. Every Record verb with a stale lease token -> ErrLeaseLost, no log row
8. RecordSuccess deletes; RecordFailure -> ready with backoff, lease
   cleared; RecordTerminal -> dead
9. RecordDelayed / RecordSuperseded / RecordDeferred leave attempts-delays
   unchanged; log row at the claim's attempt
10. Kill deads only expired inflight rows at/over budget, returns count; a
    live inflight row at budget survives
base key lease (key_lease_test.go)
11. one live lease per (group, key): second claimant busy, own token
    re-takes, expired taken over, Release with stale token false and row kept
12. compacted claim not at head -> superseded, no lease row left
13. ordered claim declines while an earlier same-key message above the
    cursor is outside the caller's own range
consume groups (group_test.go)
14. RegisterGroup idempotent, existing cursor never moved; Head placement
    sets claimed = committed = settled_head = MAX(id)
15. DeclareBindings outcomes: first installs rows; same set joins, writes
    nothing; different set with live instance waits (log row only, bindings
    kept); without live instance installs
16. DeleteGroup removes the group's claim_lease, message_key_lease,
    exception_queue, delivery_log rows; a sibling group's survive
cursoradvancer (committed_test.go)
17. AdvanceCommitted = LEAST(min lease low, min unresolved exception - 1,
    claimed); done/dead/superseded don't hold it; never moves backward
janitor (binding_log_test.go)
18. waiting-declaration sweep deletes old waiting rows except each
    declarer's newest, never installed rows, at most batchSize per stream
Dropped after audit: ForceReclaimRange, attempts-delays budget in Claim,
RenewLease, ListGroupBindingConfigLog newest-per, all deliveryconsumer.

### Timing facts (quiet machine)

Container start 2.5s per binary; race link ~5s per binary; tests <1s.
Binaries already run in parallel. Reuse-by-name tried and rejected (breaks
isolation, reaper removes it anyway). Later timings were contaminated by
the other session's janitor benchmark saturating the machine.

### Pending, not started

- `pkg/sqlstreamstest` deletion blocked: the other session's untracked
  janitor tests (`sweep_test.go`, `idempotency_key_test.go`) and
  `.bench/scratchnative` import it; `cmd/sqlstreams` conn_test and
  `.bench/reliability` measure_test use `DatabaseURL(t)`.
- Those janitor tests belong under `.tests/integration/stream` by the rule; not moved
  (another session's in-flight work).
- Worker, consume, and stream are green (stream: 18 tests, the 12 below
  plus the other session's six, ran 2026-09-10 under -race). Files:
  stream_test.go (1-6), drop_test.go (8-10), sweep_test.go (11 appended
  beside the other session's three), key_lease_test.go (12),
  compaction_head_test.go (13); numbers 7 (IsEmpty) and 14 (Vacuum) were
  cut in review. setup_test.go keeps the other session's client-based
  helpers (alias renamed to janitordatastore) and adds: newStreamDatastore
  (streams, system), declaredStream, rejectMigrationLogInserts,
  newUnregisteredStreamDatastore, registerConsumerGroup, produceMessages
  (through the produce controller, named ProducerFunc), streamTables,
  newPartitionedJanitor (janitor, orders at PartitionSize 2 with a
  "processor" group), ageMessages, commitCursor, insertDeliveryRows,
  listPartitions (bounded by the active partition: a produce creates the
  next partition in a background goroutine), listMessageIds,
  processorGroupId, claimKeyLease, expireKeyLease,
  insertEmptyCompactionHead, insertCompactionHead, listKeys,
  holdTransaction, seedMessages. seedMessages exists because a produce
  burns an id whenever the next partition is missing and the create-ahead
  runs in a background goroutine, so a produce loop at PartitionSize 2
  lands on ids 1, 3, 5, ... nondeterministically; it produces to make the
  partitions through the real path, then replaces the rows with dense
  ids 1..count. Stream registration through the stream controller logs
  "could not run register-time alert pass -- alert evidence is
  insufficient" at WARN on every fresh stream (library noise, not fixed).
  1. Register creates partition 0, the log row, and the migration baseline
     in one transaction (CHECK (false) NOT VALID on migration_log)
  2. Register again: same config no log row; changed mutable config
     replaces + one log row; different partition_size ErrStreamConfigMismatch
  3. concurrent first registrations leave one stream (errgroup, 8)
  4. Rename keeps id, moves name, one log row; ErrStreamNameTaken; (nil, nil)
  5. Delete over 101 partitions drops all ten tables, cascades log and
     consumer_group_config, sibling survives
  6. List, Get, GetById on an unregistered database are (nil, nil)
  8. DropExpiredPartitions: ttl 0 keeps all; newest-row rule; active kept
  9. DropExpiredPartitions stops at the lagging cursor; allowDrop ignores it
  10. a partition drop deletes exception_queue, delivery_log (mode on),
      compaction_head rows in its range
  11. a row sweep deletes swept rows' exception_queue, delivery_log, and
      compacted heads; unswept head kept
  12. SweepExpiredKeyLeases deletes only expired rows, batch 1 over two
  13. SweepExpiredEmptyCompactionHeads: idle empty deleted, filled and
      young kept, locked row skipped until its holder ends
  No tests: Get/GetById/GetInTx happy paths, IsEmpty, Vacuum,
  ErrStreamPartitionsRemain, ErrStreamDeclarationInterrupted,
  ddlLockTimeout, sweep batch caps and ttl guards, log lines.
  Schedule: 7 promises approved and written 2026-09-10 under a benchmark
  hold -- compiled and vetted only, never run. First step on resume:
  `cd .tests && go test -race -count=1 -v ./integration/schedule/`.
  Files under `.tests/integration/schedule/`: schedule_test.go (1, 2, 4),
  status_test.go (6, 7), due_test.go (8, 9); setup_test.go:
  newScheduleDatastore (schedules, orders stream), registerSchedule,
  newScheduleProducerDatastore, setNextScheduledAt (UPDATE now + offset),
  rejectCursorInserts, registerConsumerGroup (patterns through
  DeclareBindings), produceScheduleMessage (produce controller: name as
  message and routing key, compacted, ScheduledAt in options),
  insertDeliveryOutcome, holdTransaction. Watch on first run: #1 and #2
  compare NextScheduledAt against time.Now (an @hourly boundary within
  clock skew would flake); #6 DeepEqual on summary rows; #9 passes pgx.Tx
  as the Querier.
  1. Register writes config + cursor together; cursor insert rejected ->
     no config row (a cursorless config row is invisible yet holds the name)
  2. Register again: same values same row; new timeout keeps a due
     next_scheduled_at; new expression re-seeds it after now
  4. Suspend sets suspended; Unsuspend clears it and moves a due time past
     now; both ErrScheduleNotFound for a missing name
  6. Status lists groups with no bindings or a matching binding and counts
     succeeded / failed / superseded per group; the pending head counts nothing
  7. ListMessages newest first within the limit, scheduled_at from options,
     head flagged, outcome booleans
  8. ListDue orders due unsuspended schedules by time; future and
     suspended-due absent
  9. ClaimDue locks the row (other tx nil via SKIP LOCKED), reclaimable
     after rollback, not-due and suspended nil; Advance in the claiming tx
     removes it from ListDue and sets last_scheduled_at
  Cut in review: concurrent registrations (3), Delete (5), Suspend inside
  the produce transaction (10 folded into 9), the superseded-by pointer
  (a Go helper, unit-test material). No tests: Get/List, dbNow,
  ErrScheduleDeclarationInterrupted, ErrPayloadNotEncodable, an expression
  with no next time, controller guards, the instance's already-produced
  path; schedule.IdempotencyKey is unit-test material.
  Alert: no integration tests -- its one datastore verb
  (PartitionLockCeiling) divides server settings; classify and
  EvaluateHistory are Go over messages (unit-test material, history has
  one); Record is controller-level.
  Metric: 7 proposed, 2 trimmed (ScheduleSnapshots, SchemaVersionCounts),
  5 written 2026-09-10 under the benchmark hold -- compiled and vetted
  only, never run. Files under `.tests/integration/metric/`:
  consumergroup_test.go (ConsumerGroupSnapshot: counts by status, oldest
  unresolved ignores done/dead, own leases, head, (nil, nil) without a
  cursor), stream_test.go (ConsumerGroupSchemaVersionLag per group above
  its own cursor at one version; StreamSnapshot partitions, compacted
  flag, headless rows and oldest age), worker_test.go (WorkerSnapshots
  owner chain for system/stream/group-owned rows, live counts only
  unexpired, max_attempts over live only, unclaimed_for_secs sign),
  event_test.go (EventTimestamps by routing key and type, MIN(at) per
  (message_id, attempt)). setup_test.go: newMetricDatastore (metrics,
  orders), registerConsumerGroup, registerMetricsStream
  (__system.metrics through the stream controller), insertMessages,
  insertException (with age), insertClaimLease, setCursor,
  insertCompactionHead, insertEmptyCompactionHead, newWorkerDatastore,
  declareWorker, claimInstance, recordFailures, expireInstance,
  insertMetricEvent (payload as a Go map, pgx encodes jsonb). No tests:
  ListConsumerGroups, CurrentTime, resolveMetricsStreamId's error.
  Produce: 11 proposed, 3 cut (InTx commit/rollback is Postgres semantics,
  a batch failing at json.Marshal lands nothing trivially, store-as-produced
  is an adapter round-trip; its NULL-keys fact moved into the duplicate
  test), 8 written 2026-09-10 under the benchmark hold -- compiled and
  vetted only, never run. Files under `.tests/integration/produce/`:
  append_test.go (repeated key is a duplicate + NULL keys; failed
  ProducerFunc leaves no claim so the retry lands; compacted head at the
  winner: newest id within a rank, highest rank across, equal rank falls
  to the newer id; missing-partition heal creates only the covering
  partition; 8 concurrent appends into a missing partition all land;
  AppendMessageInTx heals inside its savepoint and the caller's earlier
  compaction_head marker row commits with the message), batch_test.go
  (batch ids ascend in pipeline order and a rerun is all duplicates;
  a batch rerun after a heal lands every message with no duplicates --
  claims roll back with the failed attempt). setup_test.go: partitionSize
  const 1000, produceTestMessage, errProducerFailed, newProduceDatastore
  (produces, orders), produceTestMessageFunc, failingProducer,
  plainAppend(key), compactedAppend(messageKey, rank), advanceSequence
  (setval on the message id sequence), partitionExists (to_regclass),
  countRows, listMessageIds, readHead. Heal tests move the sequence to
  2*partitionSize-1 and assert the partition an id landed in, never the
  id: a failed insert burns the id it drew. No tests: create-ahead (a
  background goroutine), ErrPartitionCreationBehind,
  ErrPartitionLockTimeout, the SchemaVersion guard, the claim-first
  snapshot fence (consume claim test pins it), key resolution (unit).
  Compaction: 6 proposed, 1 cut (the empty row's updated_at refresh --
  restates the CASE, nothing observable depends on it), 5 and 6 merged,
  4 written 2026-09-10 under the benchmark hold -- compiled and vetted
  only, never run. Files under `.tests/integration/compaction/`:
  head_test.go (LockHead creates the row, nil head, second tx under a
  100ms lock_timeout gets 55P03, locks after the holder commits; a
  compacted AppendMessageInTx in the locking tx fills the row LockHead
  created and GetHead reads it; GetHead and ListHeads skip an empty row
  and an unseen key, field check on one head), message_test.go
  (ListKeyMessages newest first within the limit with rank 0 and ""
  routing key on the uncompacted row; ListKeyMessagesByCreatedAt
  inclusive at both ends, created_at then id descending, with created_at
  set by UPDATE to fixed UTC instants). setup_test.go:
  newCompactionDatastore (heads, orders at the default partition size),
  newProduceDatastore, produceCompactionTestMessage, compactedAppend
  (routing key orders.updated), produceCompacted, produceUncompacted,
  lockEmptyHead (LockHead in its own committed InTransaction),
  setCreatedAt, heldTx (a pgx.Tx with Raw(), the shape LockHead takes),
  holdTransaction returning iDatastore.Tx, setLockTimeout (SET LOCAL
  lock_timeout in integer ms). The lock test waits only on the 100ms
  lock_timeout, which is the behavior under test. No tests: controller
  wrappers, the JSON adapter, ErrCompactionHeadNotFound (raised in admin).
  Migrate: 5 proposed, 1 cut (a NoTxn step -- wrapping it in a
  transaction fails loudly on CREATE INDEX CONCURRENTLY), the stream
  schema-state narrative trimmed to one read, not-registered trimmed to
  two calls, 4 written 2026-09-10 under the benchmark hold -- compiled
  and vetted only, never run. Files under `.tests/integration/migrate/`:
  step_test.go (IsLocked false, true after AcquireLock, true after a
  RunStep committed on the lock's connection, false after ReleaseLock; a
  step whose apply creates migration_log_version then fails leaves no
  index and only the baseline success row, TryRecordFailure writes the
  version-2 failure row with the error text, Version stays 1, then the
  succeeding step creates the index and Version reads 2), version_test.go
  (system: up 2 floor 0, up 3 floor 3 -> {3,3}; down to 2 -> {2,0};
  the stream owner's up-to-3 reads {3,3} on StreamSchemaState while the
  system still reads {2,0}; SystemOwner on a tableless schema and
  StreamSchemaState(404) both ErrNotRegistered). setup_test.go:
  versionIndex const, errStepFailed, newMigrateDatastore (migrations,
  system), newUnregisteredMigrateDatastore, registerStream, systemOwner,
  streamOwner, acquireLock (ReleaseLock at cleanup), transactionalStep
  (NewStep with no validate), indexExists, countMigrationRows, and the
  step applies applyNothing, createVersionIndex,
  createVersionIndexThenFail (all DDL is an index on migration_log). No
  tests: ErrStepLockTimeout (2s lock_timeout times the retry curve),
  ListStreams, the controller's range checks and step-index math, the
  registry Validate (unit-tested).
  Run order on resume: schedule, metric, produce, compaction, migrate,
  all -race -count=1. Remaining root with no directory: system
  (Register, Get, Delete).
- Shape rule added to CONVENTIONS Part 5 (Test shape): tables hold values
  only; a set of verbs is straight-line calls and checks; no funcs in rows,
  no closures, no t.Run around one verb.
- `.env` still lacks `SQLSTREAMS_TEST_DATABASE_URL`; not needed for .tests
  (Docker), only for the override.
- Nothing committed by this session.
