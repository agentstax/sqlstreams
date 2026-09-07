# Reliability lab -- research synthesis

Research for the ROADMAP item "Reliability lab -- the hour-long live run".
Six slices: Kafka's system tests and descendants, Jepsen and the
history-plus-checker lineage, Postgres-backed queue projects, load-shape
languages, Go tooling and chaos mechanics, and the maintainability of
long-running suites across the industry. Every claim below was read in a
primary source unless marked (inferred). Working docs, deleted at close-out.

## 1. The verdict in one paragraph

The design we settled (ledger + checker, scenario file, named buckets,
duplicates reported not failed) is the shape every surviving suite converged
on: Kafka's ProduceConsumeValidate (ten years old, still the template for
every Kafka durability test), Jepsen's `total-queue` checker, Redpanda's
chaos harness, WarpStream on Antithesis, hardbyte's CDC verifier. Nothing in
the field contradicts it. What the research adds is a set of specific rules
the precedents learned the hard way, listed in section 2, and a handful of
deltas to our staged plan, listed in section 9.

## 2. Rules the precedents earned

### 2.1 The correctness model is Jepsen's `total-queue`, verbatim

Source fetched: `jepsen.checker/total-queue`. Three multisets -- attempts
(invoked produces), acknowledged (committed produces), dequeues (handler
invocations). Then:

    lost        = acknowledged - dequeued          FAIL
    unexpected  = dequeued - attempted             FAIL   ("records from nowhere")
    duplicated  = (dequeued - attempted) - unexpected   REPORT
    recovered   = (dequeued ∩ attempted) - acknowledged  REPORT

`recovered` is the ambiguous-produce case: an indeterminate produce counts as
an attempt, never an acknowledgement. If it appears downstream it is
recovered (fine); if it never appears it is nothing (not lost, because it was
never acknowledged). Only acknowledged produces can be lost. Kafka's own
validator has no ambiguous bucket at all (timeouts land in `not_acked` and
the validator only checks acked minus consumed), and Tom Lane confirms libpq
gives no SQLSTATE on connection loss -- so our ledger MUST record attempted
and acknowledged as two separate facts, or a later-appearing message gets
misreported as unexpected. pgx side: a lost COMMIT reply returns a wrapped
net error with no CommandTag and `pgconn.SafeToRetry` false; the ledger
records "outcome unknown", not "failed".

One sharper case from Elle: a produce the library REJECTED (a VK error) whose
row is later delivered is an aborted read (G1a) -- a hard failure, distinct
from recovered. Our rejected-with-VK-code bucket needs that assertion.

### 2.2 Loss versus lag is settled only by heal-then-drain

Redpanda's Jepsen report: "surprisingly difficult to distinguish between
messages which are permanently lost versus simply delayed." Every precedent
resolves it the same way: heal every fault, stop producers, then drain until
a criterion, bounded by a budget that fails the run (Jepsen Redpanda budgeted
an hour; Kafka defaults 30 s per partition). The criterion matters:

- Kafka: every partition's committed cursor >= last acked offset. Its
  ReplicationTest docstring warns that stopping on `consumed == acked` exits
  early under duplicates.
- NATS 2.12.1: fetch until the last acknowledged message of EACH producer
  has been observed. This per-producer high-water mark is the cleanest
  quiesce criterion for us: per producer, its last committed idempotency key
  has a delivery_log row ending success or dead.

Jepsen makes the drain the test's responsibility: "Queues only obey this
property if the history includes draining them completely."

### 2.3 Safety checks are window-blind; liveness checks are window-aware

No core Jepsen checker relaxes a safety verdict inside a fault window --
loss, unexpected, aborted-read hold everywhere. Fault windows reach the
checker only as history rows (nemesis start/stop pairs), and are used for
liveness reading and plot shading. Redpanda writes
`injecting/injected/healing/healed` events with timestamps INTO the workload
ledger so the checker can attribute anomalies to windows; Kafka does not,
and so tolerates duplicates run-wide. For us: `run_phase` rows carry the
chaos windows; lost/unexpected/aborted-read ignore them; reclaims,
dead-letters under fail rate 0, and recovery time are judged against them.
A duplicate with no reclaim in a fault window is a bug (inferred from the
GoodJob/graphile/Oban heartbeat discussions, which all admit duplicate
execution only under partition).

### 2.4 A run that never reached the fault is `unknown`, never `pass`

The single most repeated lesson. Antithesis: "code coverage only covers
locations, while sometimes assertions cover situations" -- a `Sometimes`
that is never encountered fails. ScyllaDB's nemesis publishes exactly one
`SKIPPED` row when a precheck excludes it, so the timeline shows which
faults actually ran. Jepsen's `stats` checker returns `:unknown` unless
every op type had some `:ok`. Kafka gates faults behind `>= 5` deliveries
and refuses to stop unless acked grew in the last 30 s. Redpanda's
`progress_during_fault{min-delta}` returns `HANG`. TigerBeetle added a
permanent-fault mode because uniform healing faults "mask scenarios where a
cluster could get stuck."

Mapping: each safety bucket is `Always(count == 0)`; each chaos-reached fact
is a `Sometimes` -- ambiguous produce occurred, a lease was reclaimed inside
a kill window, a handler ran twice for one message, Postgres pause produced
a transient datastore error. A run where a scenario's `Sometimes` stays at
zero reports `unknown`. This is also the answer to the classic one-minute
dev run that "passes" because lease expiry never happened.

### 2.5 Verdicts are a lattice, not a boolean

Jepsen: `:valid?` in `{true, :unknown, false}` with priorities
`false > :unknown > true`; a checker exception becomes `:unknown`. Redpanda:
`FAILED > UNKNOWN > HANG > NODATA > CRUSHED > PASSED`. ducktape records
`FLAKY` as a status distinct from `PASS` when a `--deflake` rerun passes.
Exit codes for us: 0 pass, 1 fail, 2 unknown, 3 lab-infrastructure error
(compose/docker), so a justfile or CI distinguishes "system wrong" from "lab
broken". CockroachDB found every nightly Jepsen failure for two years before
the real bug was harness flakiness -- the fourth code is what keeps those
from being read as findings.

### 2.6 The ledger must be durable and the checker must stream

Kingsbury's in-memory histories OOM'd a 16 GB heap after about two hours; the
fix was an append-only chunked disk log and fold-based checkers. Redpanda's
Gobekli restricted itself so checking is O(n) streaming. Harry keeps no
history at all (regenerable from seed). For an hour at thousands of produces
per second the ledger is a table and the checker is SQL, which is what we
planned. See section 9 for the transport fork this raises.

### 2.7 Readers learn a test from one document, and the report quotes it

ducktape: the docstring is a numbered procedure and "is how readers learn
what a test does"; the wiki that justified ducktape called the old suite
"hard to read/understand and therefore difficult to develop new tests
against." k6's threshold line is the reporting model everyone cites:
`✓ 'p(95)<1500' p(95)=148.21ms` -- the expectation verbatim, then the
actual, then the mark, with a non-zero exit. No load tool asserts per named
phase; Litmus probes (`SOT/EOT/Edge/Continuous/OnChaos`) and Chaos Toolkit
(hypothesis before as gate, after as verdict, `deviated: true`) have the
window vocabulary but one hypothesis per run. Per-phase expectations are a
gap the lab fills.

### 2.8 Flakes get a ledger and an owner, never an auto-retry

Kafka KIP-1090: 59 of 64 trunk builds flaky in a week; the fix is a
`@Flaky("JIRA")` quarantine with a 7-day/10-build exit, over the objection
that quarantine hides tests. Kubernetes: jobs must not auto-retry, `[Flaky]`
quarantine with a mandatory critical issue, retirement after three months
dormant. Postgres keeps a wiki of about 97 known buildfarm failures with
symptom, log links, and thread. roachtest's admission gate is sociological:
"is my team willing and able to own this test going forward?" Every roachtest
carries `Owner`, `Timeout`, `NonReleaseBlocker`. etcd requires that "the
latest version of the robustness framework remains capable of reproducing
old bugs" -- a named reproduction command per historical bug.

### 2.9 The oracle is a ledger join, not a model checker

Gemini diffs against a second real cluster; Harry and etcd use a model plus
Porcupine; Elle finds Adya cycles. All agree the oracle must be written from
the contract, not the implementation (Dropbox's Storage Watcher was
"implemented by someone who wasn't involved in building the storage system").
For an at-least-once queue the contract is set membership plus per-key
order, so the ledger join is the right primary oracle; a per-key monotonic
check on the consumer side (WarpStream's producer-id plus sequence) covers
ordered/exclusive semantics when the compacted-topic stage arrives.
Redpanda's Jepsen response is the cautionary tale for skipping this: without
a written model, findings became "bug vs by-design" debates.

## 3. Scenario language

### 3.1 Vocabulary

Best cold reads: Gatling's `rampUsersPerSec(a).to(b).during(d)` and
`nothingFor(d)`; Artillery's `arrivalRate / rampTo / pause`. The keyword
names the quantity and the unit rides on the number. Worst: Tsung's separate
`unit=` attributes, Artillery's bare-number-means-seconds, k6's
`ramping-vus` versus `ramping-arrival-rate` sharing one `stages` syntax
where `target` means two different things (a forum thread is titled exactly
that confusion). k6 also leaks generator resourcing (`preAllocatedVUs`,
`maxVUs`, `dropped_iterations`) into the scenario.

Recommended grammar, grounded line by line in those precedents:

    [shape]
    warm:     steady 200/s 2m
    surge:    ramp 200/s -> 2000/s 10m         # linear, Gatling rampUsersPerSec.to.during
    quiet:    pause 30s                        # Gatling nothingFor, Artillery pause
    flood:    saturate 64 in flight 5m         # closed loop, Vegeta -rate=0 -max-workers
    cooldown: steady 200/s 5m

    at 0m    consumers 3
    at 12m   consumers 0
    at 13m   consumers 8

    at 5m    kill consumer 1                   # Pumba verbs
    at 8m    pause postgres 20s

Named producer phases run back to back and become the report's row keys.
Consumer counts and chaos use `at <offset>` (FIS `startAfter` and Chaos Mesh
serial trees exist to order things with no natural sequence; a lab timeline
has one). `[expect]` scopes with the window words: `during flood`,
`after quiet`, `throughout`. Steps are a chain of `steady` lines until a
scenario needs more.

### 3.2 Open loop, closed loop, and what saturate means

Brooker: closed benchmarks "slow their load down when latency goes up" and
can underestimate p99 by 25x; wrk2 measures from the scheduled send time so
a stall shows in every request that was due during it. Artillery: with a
closed loop "a random 5 second stall is hidden from us." For a producer that
blocks on commit, `steady N/s` must be open loop -- a pacer schedules
produce N at t0 + N/rate regardless of whether N-1 returned, latency is
measured from the scheduled instant, and a produce that never got scheduled
because in-flight was capped is counted (k6 `dropped_iterations`), never
silently skipped. Otherwise a 5 s Postgres pause at 200/s hides a thousand
messages of queueing.

`saturate` must be bounded by concurrency, never bare "as fast as possible"
(Vegeta warns `-rate=0` unbounded can crash the generator; Fortio
`-qps 0 -c N`; Gatling `constantConcurrentUsers`). `saturate 64 in flight`
is 64 goroutines each looping produce -> commit -> produce; achieved rate is
the measurement, latency is service time only, and the report labels the
mode so the two are never compared.

### 3.3 File format

Hand-rolled line parser, on the go.mod and testscript precedents ("each line
parses into space-separated command words", `#` comments, failures as
`FAIL: script.txt:3:`). TOML/YAML/HCL/CUE each cost a dependency and YAML
has the Norway problem. Pitfalls to design around: `time.ParseDuration` has
no `d` and `m` is minutes, so require a unit on every duration; a rate needs
its own token (`200/s`) so `200` can never be read as a rate; every parse
error carries `file:line:`; an unrecognized keyword lists the closed set
(the CONVENTIONS fix rule); validate the timeline (offsets within the sum of
phase durations) before anything runs. Gherkin's failure mode is the one to
avoid in `[expect]`: abstract expectations with no concrete values.

## 4. Chaos mechanics for a Go author

- **Docker: shell out.** `github.com/docker/docker` is deprecated since
  Docker v29 (April 2026) and govulncheck-flagged; `moby/moby/client` has 24
  requires including otel; testcontainers-go's compose module has 116. A
  ~100-line `os/exec` wrapper over `docker compose ps --format json` (JSON
  Lines since Compose 2.21.0, decode in a loop), `up -d --scale svc=N
  --wait`, `docker kill -s / pause / unpause / stop -t` costs zero deps.
  `compose kill` has no `--index`: target one replica by container id from
  `ps`. The conductor needs the docker socket mounted.
- **Network faults without CAP_NET_ADMIN: Toxiproxy** as a compose service
  between roles and Postgres. Toxics: `latency`, `timeout` (0 holds
  forever), `reset_peer`, `limit_data`, `slicer`, `bandwidth`,
  `enabled:false` = hard down; each has `stream` and `toxicity`. Skip its Go
  client (pulls mux, prometheus, zerolog, urfave); the API is five JSON
  endpoints. Pumba `netem`/iptables need rootful NET_ADMIN. Unverified:
  whether a toxic applies to already-open connections (source says yes;
  probe before relying on it).
- **Ambiguous commit, most deterministic first.** (a) wrap the `net.Conn`
  from `pgconn.Config.DialFunc` so the Read after a COMMIT write returns
  `io.ErrUnexpectedEOF` -- aimed at one commit, no proxy, no root (design,
  inferred); (b) Toxiproxy downstream `timeout` toxic added just before
  COMMIT -- repeatable under load, not aimed; (c) `pg_terminate_backend`
  mostly yields clean `57P01` FATALs, rarely ambiguity; (d) SIGKILL or pause
  of Postgres makes everything in flight ambiguous at once. The library
  already names the consumer-side case: `common.ErrCommitConfirmationLost`
  (VK0019). The produce-side case has no code; it is a pgx-level error the
  ledger records as unknown.
- **Postgres-side knobs, all per-session `SET`.** `statement_timeout` and
  `lock_timeout` cancel; `idle_in_transaction_session_timeout`,
  `idle_session_timeout`, `transaction_timeout` (PG17) terminate the session
  -- the cheapest "connection dropped mid-lease". `pg_terminate_backend(pid,
  timeout_ms)` on PG14+ blocks until dead so the conductor can assert the
  kill landed. Official image `STOPSIGNAL SIGINT` = fast shutdown, so
  `docker stop` is clean and `docker kill -s SIGKILL` forces crash recovery.
  `docker pause` is the cgroup freezer: TCP stays up, nothing errors until
  unpause or a client deadline -- the right tool for lease-expiry tests.
- **The `synchronous_commit = off` trap.** An acknowledged COMMIT can be lost
  on SIGKILL within 3x `wal_writer_delay` (600 ms default). A SIGKILL
  scenario under that mode produces true "acknowledged then lost" rows. The
  verdict record must carry the mode or the checker reports false loss. The
  repo's own bench records on synchronous_commit are the reference.
- **Compose trap.** `depends_on: condition: service_healthy` with
  `restart: true` restarts dependents when Postgres restarts -- disable it
  for chaos roles or a Postgres restart also restarts the consumers.
  `--exit-code-from` is known flaky with long logs (compose #11493, #9778);
  prefer `up -d --wait` then `run --rm conductor` then `run --rm checker`.
- **Self-inflicted crashes.** `-crash-after-handler=N` calls `os.Exit` with
  a distinct code inside the handler after success and before the delivery
  is recorded; `-crash-in-batch=N` exits between the Nth append and the
  flush. Server-side indistinguishable from SIGKILL, but aimed at an exact
  code point. Solid Queue is the only Postgres job queue found with
  TERM/QUIT/KILL integration tests; hardbyte's harness alternates
  `kill_worker` and `start_worker` and runs `pg_terminate_backend` at 2/s
  against `state IN ('active','idle in transaction')`.
- **pgxpool after a Postgres restart** does not ping on Acquire; expect one
  wave of closed-connection errors per pool, then recovery. Count them,
  never fail on them.
- **Rate generation.** `golang.org/x/time/rate` is its own module (a new
  dep, zero transitive). Vegeta's `Pacer` interface (Constant, Linear, Sine)
  is about 40 lines to hand-roll; go-ycsb #26 is the fixed-schedule
  goroutine fix for coordinated omission.

## 5. Records and reports

Jepsen: `store/<test>/<timestamp>/` with `results.edn` (one map per named
checker, counts AND sampled offending values, merged tri-state at top),
`history.txt`, `timeline.html`, latency and rate plots with fault windows
shaded, and a `latest` symlink. ducktape: `results/<session>/report.{txt,
json,xml,html}` plus per-test dirs with service logs. k6: one struct, two
renderings (stdout table and `summary.json`). etcd: a `report/` directory
with data dirs and per-client JSON histories, and a make target per known
bug. Yuan 2014: median 824 log lines per failure, so the verdict must point
at the window, not the log.

In-repo precedent: `tools/compat/main.go` already runs a `testing.T`-free
lab with a declared `-expect` and `os.Exit(1)`. The record for us: scenario
name, start, duration, `synchronous_commit` mode, library version, effective
time scale, per-check `{status pass|fail|unknown, expected, actual, first N
witness ids}`, per-phase counts, per-fault `ran|skipped|failed`, artifact
dir. Written as JSON to `results/<scenario>/<timestamp>/` and rendered as
the scenario file with an `actual` column.

## 6. Time scaling

Jepsen scales `--time-limit` and nemesis intervals while leases and
heartbeats stay absolute; TigerBeetle scales by simulating time, which we
cannot. The honest scale-down is shortening the fault schedule and phase
durations, never the durations that feed leases, Postgres timeouts, or pool
health periods. A one-minute dev run must still cross at least one lease
expiry and one reclaim, or its fault `Sometimes` reads `unknown`. Print the
effective durations in the start line; cap the minimum so a scenario whose
intervals fall under about two seconds is recognized as a different test.

## 7. Failure classes this lab is aimed at

From the Postgres-queue bug catalogue (River, Oban, graphile-worker, Solid
Queue, GoodJob, pg-boss, Celery, Trigger.dev, hardbyte's sweep). Every
Postgres job queue checked ships only unit and integration tests; River's
changelog shows six rescuer/completer fixes in 2026 alone, all from user
reports; hardbyte's audits found every chaos "failure" was an adapter that
did not reconnect after `57P01`, and handlers that "always return nil" so
retry paths were never measured.

| Class | Precedent | Caught by | Needs |
| --- | --- | --- | --- |
| Dead worker leaves rows claimed forever | Oban #430, Solid Queue #542, pg-boss #250, GoodJob #1249 | undelivered after drain | SIGKILL consumer mid-handler |
| Time-only rescue duplicates a live job | Oban.Lifeline docs, graphile Pro, River #1105, Celery #9963 | duplicates; reclaims outside windows | slow handler past lease, or SIGSTOP/pause past expiry |
| Ambiguous commit | Tom Lane on libpq; River 0.27.0 rollback ctx | attempted vs acknowledged ledger; recovered bucket | DialFunc fault, toxic, or Postgres kill timed to commit |
| Rejected produce whose row appears | Elle G1a | aborted-read bucket | none |
| Keyed insert drops a request under contention | graphile #580 | acknowledged with no message row | hot keyspace, no chaos |
| Poison pill / worker crashes because of the job | Solid Queue #277 (fail), Sidekiq super_fetch (requeue, dead after 3) | a declared bucket, never loss | handler that exits |
| Stall with nothing lost | Oban #493 idle-window bug | undelivered plus recovery bound | idle phases in the timeline |
| Client exits on connection loss | hardbyte adapters, 0% recovery | recovery-time bound | Postgres restart / backend kill |
| SKIP LOCKED contention, bloat | pgsql-hackers 80-core thread, pgmq past 16 workers | rate/latency assertion, not ledger | sustained pressure plus an idle-in-transaction holder |
| Lost on a graceful shutdown that was not | Sidekiq #4635 | loss | SIGTERM with grace shorter than handler |

Vulkan-specific mapping: range quarantine after max reclaims
(`consume.EventRangeQuarantined`) is the poison-pill bucket and needs its
own name in the partition; `EventLeaseReclaimed` (VK0026) is the reclaim
count the window-aware check reads; `topic.DeliveryLogModeAll` is what makes
"every eligible message ends success or dead" checkable at all.

Taxonomy support: Yuan 2014 -- 92% of catastrophic failures are mishandled
signalled errors, 98% manifest on three or fewer nodes, 77% reproducible in
a unit test; Alquraan 2018 -- 88% of partition failures are triggerable by
isolating one node, 21% leave lasting damage after heal. For a single
Postgres instance, partitions and clock skew reach the library only as
connection loss, statement timeout, and restart, so the error-path class
dominates and is catchable with kill, pause, and terminate. Disk faults
(NATS lost 49.7% on a single-bit error) reach us only through
`synchronous_commit` and crash recovery.

## 8. DST, Antithesis, and what Docker chaos can and cannot reach

DST advocates state the limit themselves: it "cannot test the integration
between your system and external systems." For a Postgres-backed library
that integration is the thing under test, so `docker kill/pause` plus
killing our own processes is the right instrument for ambiguous commits,
lease expiry and reclaim, duplicate invocation, and re-registration. It
cannot reach fsync loss (NATS used LazyFS), clock skew (only a real VM with a
real clock; containers cannot), or fast in-process races (the race
detector's job). WarpStream's data race found in 233 s after "10s of
thousands of hours" of race-detector CI is the argument for Antithesis
later; their bug discovery plateaued at about 160 simulated hours. The
transferable DST lesson today is the `Sometimes` discipline of section 2.4.
No Antithesis post on message queues specifically was found.

## 9. Deltas to the staged plan

Changes the research argues for, against what we settled in discussion.

1. **Two facts per produce, not one.** The ledger records `attempted` before
   the call and `acknowledged | rejected(code) | unknown` after. Rejected
   rows get the aborted-read check. (2.1)
2. **Tri-state verdict from day one.** `unknown` when a scenario's
   chaos-reached facts never fired or the checker itself errored; exit code
   2; lab-infrastructure failures exit 3. (2.4, 2.5)
3. **`run_phase` gains `ran | skipped | failed` per chaos action**, and the
   report's fault timeline prints it. (2.4)
4. **Quiesce criterion is per producer**: last committed key delivered,
   bounded by a budget that fails the run. Never `delivered == committed`.
   (2.2)
5. **Producers are open loop with a pacer** measuring from scheduled time;
   `saturate` is bounded in-flight and labelled closed loop. (3.2)
6. **The record carries `synchronous_commit`**, and a SIGKILL-Postgres
   scenario is only valid under `on`. (4)
7. **A reproduction scenario per bug the lab finds**, kept forever, on the
   etcd rule. (2.8)
8. **Stage 2 gets the DialFunc fault** rather than waiting for Toxiproxy in
   stage 4; it is the one deterministic way to aim at a single commit and
   costs no dependency. (4)
9. **Per-key monotonic check** lands with the compacted-topic stage, on the
   WarpStream pattern, rather than a model checker. (2.9)

One fork the research raises, with a pick. Every precedent writes the ledger
to a local append-only file per process (Kafka JSON lines, Redpanda TSV,
Jepsen's chunked disk log) and loads it for checking, because writing the
ledger into the system under test doubles its write load and perturbs the
measurement. Our settled design writes ledger rows straight into a separate
schema in the same Postgres. At an hour of `steady 2000/s` that is roughly
fourteen million extra row writes on the instance under test. Pick: keep
the ledger schema and the SQL-join report, but make the transport a local
JSON-lines file per role, `COPY`'d into the ledger schema by the checker at
quiesce. The container volume survives `docker kill`; the checker is
unchanged; the cost is one volume mount and about sixty lines. v1 may write
directly if the dev rates are modest, provided the row shape is the file's
row shape so the switch is transport only.

## 10. Not found or unverified

Confluent's nightly results site was unreachable; temporalio/canary 404s;
Stripe Bouncer and Uber uChaos have no primary sources; whether a Toxiproxy
toxic applies to open connections is read from source, not docs; the
recovery-time bound has no precedent as a numeric assertion -- every system
plots ops/s against fault and heal markers and reads it by eye, so ours is
new and should start advisory (hardbyte's definition: rate back to pre-fault
median AND lag back in band).

## Sources

Kafka and descendants: apache/kafka `tools/.../VerifiableProducer.java`,
`VerifiableConsumer.java`, `tests/kafkatest/utils/util.py`
(validate_delivery), `tests/kafkatest/tests/core/replication_test.py`,
`transactions_test.py`, `trogdor/README.md`,
`RoundTripWorkerBase.java`; cwiki KIP-1090 and "System Test Improvements";
KAFKA-18194, KAFKA-10295, KAFKA-10274; confluentinc/ducktape
`service.py`, `runner_client.py`, `reporter.py`; redpanda-data/kgo-verifier;
redpanda-data/chaos (`abstract_single_fault.py`, `consistency.py`,
`result.py`); redpanda.com/blog/validating-consistency; linkedin/kafka-monitor.

Jepsen lineage: jepsen-io/jepsen `checker.clj` (set, total-queue,
merge-valid), `tests/kafka.clj`, `nemesis/combined`, `checker/perf`,
`doc/lxc.md`, tutorial 04-08; jepsen.io analyses redpanda-21.10.1,
bufstream-0.1.0, nats-2.12.1; aphyr.com posts 293 (Kafka), 315 (RabbitMQ),
365 (10^9 operations); jepsen-io/maelstrom `doc/results.md`; jepsen-io/elle;
anishathalye/porcupine; cockroachlabs.com/blog/jepsen-tests-lessons.

Antithesis and DST: antithesis.com docs (Go SDK assert, sometimes
assertions, properties); warpstream.com DST-for-our-entire-SaaS; etcd.io
blog autonomous testing with Antithesis; notes.eatonphil.com DST;
tigerbeetle.com VOPR and simulation-testing-for-liveness; sled.rs/simulation.

Maintainability: cockroachdb roachtest README and `registry/test_spec.go`,
issues #43920 #134104; etcd-io/etcd `tests/robustness/README.md`, issues
#14045 #13637 #5499 #14826; scylladb/scylla-cluster-tests `docs/nemesis.md`,
scylladb/gemini `docs/architecture.md`, scylladb/argus; Cassandra Harry
blog; dropbox.tech Pocket Watch; slack.engineering Disasterpiece Theater;
kubernetes/community `flaky-tests.md`; wiki.postgresql.org
Known_Buildfarm_Test_Failures; testing.googleblog.com flaky tests; Yuan et
al. OSDI 2014; Alquraan et al. OSDI 2018; asatarin/testing-distributed-systems.

Postgres-backed queues: temporalio/omes (README, `loadgen/scenario.go`,
`scenarios/throughput_stress.go`, `ebb_and_flow.go`), temporalio/maru,
temporal.io stress-testing post, uber/cadence canary;
hardbyte/postgresql-job-queue-benchmarking (`docs/method.md`,
`bench_harness/phases.py`, ADR-001, `docs/cdc-harness-design.md`, 2026-05-09
sweep audits); riverqueue/river CHANGELOG, #1105, #601; oban-bg/oban #1037
#430 #493, Oban.Lifeline docs; graphile/worker #580 #222, Pro recovery docs;
rails/solid_queue `forked_processes_lifecycle_test.rb`, #542 #159 #422 PR
#277; bensheldon/good_job #831 #1249 #177 #480; timgit/pg-boss #250 #522;
que-rb/que README; sidekiq Reliability wiki, #4635; celery #5935 #9963;
trigger.dev incident 2025-09-26; Tom Lane on connection loss
(postgresql.org message-id 26881.1555425074); pgsql-hackers SKIP LOCKED
thread; richyen.com postgres_job_queue.

Load shape: grafana.com k6 docs (open-vs-closed, executors, arrival-rate VU
allocation, dropped iterations, thresholds, end-of-test); docs.gatling.io
injection and assertions; artillery.io test-script, ensure, workload-models;
tsenart/vegeta README and `lib/pacer.go`; giltene/wrk2;
brooker.co.za open-closed-omission-collapse; fortio; ghz load schedules;
Tsung conf-load; chaostoolkit.org experiment, run-flow, journal;
chaos-mesh.org scheduling and workflow; docs.litmuschaos.io probes;
alexei-led/pumba; AWS FIS action sequence; rogpeppe/go-internal testscript;
pkg.go.dev time.ParseDuration; strictyaml Norway problem; cucumber
anti-patterns; openmessaging/benchmark workloads; pingcap/go-ycsb #26.

Tooling: moby/moby discussion #52404 and `client/go.mod`;
testcontainers-go `modules/compose/go.mod`; ory/dockertest; docker compose
`up`, `ps`, `kill` references, compose issues #10958 #11493 #9778, container
`pause` reference; Shopify/toxiproxy README and go.mod; pkg.go.dev
pgx/v5 pgconn (SafeToRetry, ErrConnClosed, CheckConn), pgxpool;
postgresql.org functions-admin, runtime-config-client, wal-async-commit;
docker-library/postgres Dockerfile (STOPSIGNAL); golang.org/x/time/rate;
uber-go/ratelimit; maelstrom results.md; ducktape run_tests; k6
custom-summary.
