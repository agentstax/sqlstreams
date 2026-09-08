# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

## Benchmark-recording pipeline on the reliability lab [0687] [0696] [0697]

`bench/reliability` is the Postgres-bound benchmark harness. A benchmark is
a scenario: the same compose stack, roles, pacer, records, checker, and
verdict. Nothing is built beside it -- no shared driver package, no
histogram dependency, no second record shape. Each chunk is a review
checkpoint, not permission to expand scope; the smallest change to the
lab that carries the rule wins.

Settled 2026-09-07, recorded in [0711]:

- Every run measures; a scenario declares only expectations. Latency and
  throughput come from the record rows on every run. Latency expectations
  are `report` for now, so `Want` keeps its two values.
- Guards are checks: `backlog_bounded`, `schedule_kept`,
  `generator_headroom` join the `Check` set with want `0`, counted as
  seconds the guard was violated. Sustainable max is the highest stepped
  phase whose guards held.
- The ledger records every produce (option A); the headroom guard is what
  shows if the ledger itself becomes the limiter. Sampling is the fallback,
  not the default.
- A fourth role, `observer`, samples server counters at 1 Hz into its own
  record file; the checker loads it like the others.
- The recipe writes the fingerprint (the container cannot see git); the
  checker merges it into the verdict.
- One tracked append-only line per run in `results/<scenario>/runs.jsonl`;
  the timestamped run directories with raw records stay untracked.
- Compose takes the `container.sh` spec: `postgres:18.4`, CPU and memory
  caps, the GUCs, a named volume for PGDATA.
- `Topic` becomes a list of topic declarations, each with its groups and
  producer phases; replicas come from `docker compose --scale`.
- No directory rename, no "cell" anywhere, `just reliability-lab` stays
  the one entry point. Tier 1 (`go test -bench` + benchstat) covers CPU
  paths and is untouched by the lab.

Verification per chunk: `cd bench && go build ./... && go vet ./... && go
test -race -count=1 ./reliability/...`, then `just reliability-lab dev`
green, then the chunk's own sabotage. A green run is trusted only after
the sabotage turns it.

### 1. Environment and fingerprint

Landed 2026-09-07: dev scenario green on 18.4 through the recipe; both
sabotages (dirty tree, `synchronous_commit` set off by ALTER DATABASE)
visible in `verdict.json`. `shm_size: 1g` added beyond the `container.sh`
spec so parallel workers never hit the 64MB default.

- [x] Move the `container.sh` spec into `compose.yaml`'s postgres service:
  `postgres:18.4`, `--cpus`/memory caps, `shared_buffers`, `max_wal_size`,
  `pg_stat_statements`, `track_io_timing`, `max_connections`, and a named
  volume for PGDATA (never the container layer). Drop `restart` for
  nothing under test.
- [x] The recipe writes `fingerprint.json` into the run's results directory
  before the checker runs: library sha plus dirty flag, Go version, host
  CPU model, cores, RAM, OS, and the compose image tag. The checker reads
  it, adds the Postgres version and the settings it can `SHOW`
  (`synchronous_commit` moves into this block), and writes the whole
  block into the verdict.
- [x] Sabotage: run with a dirty tree and a changed GUC; both must be
  visible in `verdict.json`.

### 2. Latency and throughput from the records

Landed 2026-09-07: dev shows produce p50 2.7ms / p99 7.8ms and end-to-end
p50 264ms / p99 502ms at 200/s (the claim poll rate is visible in the
end-to-end spread). Sabotage of 1 in 100 `scheduled_at` values by 5s moved
p99.9 to 5.00s and failed `schedule_kept` on 61 seconds. Two departures
from the plan below: the slip tolerance is a fixed 100ms rather than one
pacer interval (at 200/s one interval is 5ms, inside timer jitter), and
`schedule_kept` is declared per scenario, not an invariant, since a chaos
scenario that pauses Postgres slips by design.

- [x] Produce latency is `at - scheduled_at` on the committed row;
  end-to-end latency is the handler's first success `at` minus
  `scheduled_at`, joined on message id. Per-second throughput is committed
  rows bucketed by `at`. All three computed by the checker datastore in
  SQL over `lab.*`, p50 / p90 / p99 / p99.9 / max, per phase and whole run.
- [x] The verdict gains a `measure` block: the percentiles, the per-second
  throughput series, and the achieved rate per phase against the declared
  rate. The report prints the percentiles beside each phase line.
- [x] `schedule_kept` check: seconds in which any attempted row's `at`
  trailed its `scheduled_at` by more than one pacer interval (the in-flight
  permit blocked, the run went closed-loop). Want `0`, joins `Invariants`
  only if every existing scenario can hold it; otherwise declared per
  scenario.
- [x] Sabotage: shift `scheduled_at` on a handful of rows in a record file;
  the percentiles and `schedule_kept` must move.

### 3. The observer role

Landed 2026-09-07: dev reports wal 1171 B/msg, 13.8 records/msg, 3.0
transactions/msg at 200/s with delivery log mode all (the consumer side is
inside those numbers). Sabotages: backlog samples rewritten to grow failed
`backlog_bounded` (+168/s); producer stats rewritten to 190% of a 2-CPU cap
failed `generator_headroom` on 18 samples. A flood at 20000/s held its
schedule with the producer at 44% of one core and ended unknown on the
drain budget, so a saturating run cannot pass; `just reliability-lab` gained
a `drain_budget` parameter for it. Departures from the plan: guards count
phases (backlog slope over 5% of the declared rate) and container samples
(about every 3s), not seconds. Recipe fixes found on the way: just's
dotenv-load leaked the dev database's POSTGRES_* into compose (now unset in
the recipe); the exit trap's failing `kill` skipped teardown under `set -e`
(now `|| true`); the stats sampler must be waited for or its last line
races the checker's load.

Finding for chunk 8: `CountLost` counts an idempotent duplicate as lost,
since a duplicate produce reports id 0. Match duplicates by key.

- [x] `-role observer`: its own compose service, samples at 1 Hz into
  `observer.sample.jsonl`: `pg_stat_wal` (records, fpi, bytes),
  `pg_stat_checkpointer`, `pg_stat_database` (xact_commit, deadlocks,
  blks_hit/read), and per group the cursor lag
  (`max(message_log.id) - committed`). Read-only on the library's tables,
  through `pkg/topic`'s table-name funcs. Starts with the consumer, stops
  after the producer exits.
- [x] The checker loads `lab.observer_sample` and writes before/after
  deltas into `measure`: WAL bytes, records, and FPI per committed message,
  checkpoints in the window, deadlocks. The raw series stays in the run
  directory.
- [x] `backlog_bounded` check: seconds in which cursor lag exceeded its
  value one phase earlier by more than the phase's declared rate (the
  scale bench's `diverging` verdict, standardized). Want `0`.
- [x] `generator_headroom` check: the recipe samples `docker stats` for the
  producer and consumer containers into the results directory; the checker
  counts seconds either container sat above 80% of its CPU cap. Want `0`.
- [x] Sabotage: a scenario whose declared rate exceeds what the container
  can produce must fail `generator_headroom` or `schedule_kept`, never pass.

### 4. Recording and reps

Landed 2026-09-07: two reps of dev through the recipe wrote two lines of
about 3KB each to `results/dev/runs.jsonl`, both pass; `just
reliability-report dev` prints one identity with the medians. Sabotage on a
scratch copy: a second library sha split into its own identity marked "no
rep", and a p50 rewritten to 100ms moved the two-run median to 51ms. The
run line also drops the throughput series, not only phases, since an hour
of per-second samples is ~100KB per tracked line; it stays in the run
directory's verdict.json.

- [x] The checker appends the verdict, minus the `phases` array, as one
  line to `results/<scenario>/runs.jsonl`. Un-ignore that file; the
  timestamped directories stay ignored.
- [x] `just reliability-lab scenario time_scale reps`: the stack is rebuilt
  once, the scenario runs `reps` times fresh (compose down -v between), the
  exit code is the worst verdict.
- [x] `-role report -scenario <name>`: prints a table from `runs.jsonl`
  grouped by fingerprint identity (library sha, image tag, GUCs), median
  of reps per number, and "no rep" where a scenario ran once. Summaries
  come from this, never by hand.

### 5. Multi-topic scenario shape and the first workload

In progress 2026-09-07. Shape landed: `Topics []TopicDeclaration` each
with its groups; the producer phases stay at scenario level and every
topic runs them, the rate per topic (the printer says "per topic" past one
topic). Records carry the topic; per-topic checks (lost, unexpected,
recovered) sum once per topic, per-group checks once per group. Each phase
row now carries its own guard reading (backlog slope, slips, headroom
breaches, held), so a ladder whose run-level guards are declared report
still reads its sustainable rung off the phase rows. Sabotage: multitopic-4
at 1/16 scale with two replicas, one message_log row of orders-3 deleted,
failed lost with the example naming the topic -- and exposed that the
per-topic checks had been summed per group (fixed). The consumer's fixed
hostname is gone so `--scale` replicas name their records by container id.
`golang.org/x/sync` is now a direct import of the bench module; its
`// indirect` marker in bench/go.mod is stale and left for a hand edit,
since `go mod tidy` is not run there.

- [x] `Scenario.Topic` becomes `Topics []TopicDeclaration`: name, delivery
  log mode, groups. The producer phases stay scenario-wide, rate per topic.
  The printer lists every topic and group; the `.scenario` files regenerate
  and their test diffs. Producer and consumer roles run every topic
  concurrently.
- [x] Replicas: `docker compose up --scale consumer=N` through the recipe's
  `replicas` parameter; records are named by container id.
- [x] `multitopic-1`, `-4`, `-16` ladders declared: the same total rate
  stepped 2000 -> 16000/s over the topics, two groups per topic in batches
  of 100, sixteen producer batch workers per topic, guards declared report
  and read per rung off the phase rows. Run at time scale 0.25 (30s rungs)
  on 2026-09-08 at the user's request for short runs; the full-length
  rungs that span a checkpoint, three reps, and the rate runs at half the
  held maximum remain.
- [x] Limiter named: `results/multitopic/RESULTS.md`. The durable ceiling
  is ~8000/s total at every topic count, commit latency under
  `synchronous_commit` on (the off cell held 16000/s at sub-ms p50); no
  container near its cap. Open question for the library: a batch waits
  ~200ms on vs ~100ms off while fdatasync costs 0.45ms.
- [ ] Full-length rungs spanning a checkpoint, three reps, and
  `multitopic-<n>-rate` runs at half the held maximum for the latency
  spectrum. Deferred while runs stay short.

Harness changes the ladder forced, all landed: the pacer's in-flight cap
scales with the rate (2s of it, floor 256); a group declares its
`BatchLimit`; the scenario declares `ProducerBatchConcurrency`; the
recipe gained `sync` (a labelled diagnostic cell via ALTER DATABASE) and
a pre-run `down -v --remove-orphans`, since a killed run's one-off
container kept the records volume alive into the next run; the checker
measures the produce side before draining; each phase row carries the
median CPU of the postgres, producer, and consumer services; the verdict
carries the declaration and time scale, and the report groups by them.
Three lines from harness-bug and sabotage runs were removed from the
untracked-so-far `runs.jsonl` files before they are first committed.

### Sustainable throughput tuning (agreed 2026-09-07)

- Target this Mac first: 1 KB messages, one group, minimal handler,
  automatic producer batching, default failure-only delivery logging.
- Keep durable commits, fsync, full-page writes, and autovacuum enabled.
  Tune configuration first; propose library/SQL changes after profiling.
- Total benchmark storage budget raised by the user to 100 GB (2026-09-08);
  preserve 40 GiB host free space.
  Include records, checker imports, WAL, and temporary files. Measure
  growth in short probes before choosing retention and longer windows.
- Start with 10–30s probes, then 1–2m candidates, then three 15m windows
  after warmup, extending until maintenance has actually run. Require
  no growing producer or consumer queue and p99 end-to-end <= 1s.
- Compare native and Docker Postgres under matched versions/resources.
  No cloud or hardware purchases in this scope.
- [x] Correct topic/group latency accounting and failure-only checking.
  Database-backed fixtures pass; removing one handler record failed with
  exactly one undelivered message. COPY imports now ANALYZE before checks.
- [x] Add the automatic-batching 1 KB workload and record its settings.
  Scenario-file input varies configuration without recompiling the harness.
- [ ] Short storage-bounded probes; configuration sweeps and profiles.
  2k/4k/8k/16k per second passed 30s probes; 32k failed twice with
  131/57 committed messages lacking handler calls despite cursor progress.
  Full second-run evidence is retained under
  bench/reliability/results/throughput/evidence/20260908T020110Z/.
  Serial producer batches avoided missing calls in two diagnostics, but
  still failed latency/backlog at 32k. [0714]/[0715] fix a deterministic
  reproduction of skipped ids on the active cursor-consumer path and commits empty
  claims so later polls can use their observations. Two repeated 32k/s
  runs handled all 1.92m messages without missing/duplicate deliveries,
  but backlog still grew. Configuration sweeps resumed: producer/consumer
  batches, queue size, polling, process counts, CPU allowances, Go GC and
  thread settings, partition size, PostgreSQL JIT, and record storage.
  All exception consumers are suspended; the archived delivery consumer
  is excluded. Two producers at an aggregate 64k/s produced and consumed
  all 1.92m messages in a 30s probe. Reducing batch concurrency to one
  per producer yielded a 63,629/s hold, p99 411ms and no growing hold
  backlog or schedule slips (startup still failed backlog). The one-minute
  48k/s repeat had p99 985ms but backlog +398/s: not sustainable despite
  the harness PASS. A 96k/s probe overloaded. No maximum established.
  See bench/reliability/results/throughput/RESULTS.md.
  Long runs also need bounded recording/import storage; the current
  all-records/all-messages method exceeds 20 GiB before 15m at high rates.
- Native scratch exploration (2026-09-08): reliability suite paused at the
  user's request. Code: bench/scratchnative/main.go; runner: run.py there.
  Native PostgreSQL 17.9 on 127.0.0.1:55439; session path is in
  /private/tmp/vulkan-native-session-path.txt. Separate producer/consumer
  processes keep counters, duplicate bitsets and 1ms latency histograms
  in memory, plus CPU profiles and one-second resource/pool samples.
  Runtime consumer settings are logged at Info; actual producer batches
  are counted by transaction id after production. All exception workers
  are suspended before measured production; native durability remains on.
  Initial 20s runs reached 54.4k/s (64 callers) and 63.5k/s (256 callers),
  end-to-end p99 <=43ms / <=53ms. With 4096 callers and four batch
  transactions, 2m messages completed production in 14.41s (~138.8k/s),
  end-to-end p99 <=124ms, zero errors/duplicates; actual batch average
  974.66 and maximum 1000. These are short scratch probes, not a
  sustainable maximum or a controlled Docker/native comparison.
  A further 20s run with 8192 callers / eight batch transactions handled
  all 2,738,475 messages (~136.5k/s), p99 <=744ms, zero errors/duplicates.
  The four-transaction setup is the better initial candidate. Disposable
  databases were dropped; native cluster and profiles remain available.
  PostgreSQL 18.6 (latest stable checked against postgresql.org) is now
  installed and active at the same scratch port. PostgreSQL 17.9 is stopped;
  its scratch cluster remains available with page checksums enabled to
  match 18.6. Two alternating 2m-message runs per version, identical
  scratch executable / 4096 callers / four batch transactions / durable
  settings: 17.9 130,119 and 126,082/s (p99 <=161/234ms), 18.6 128,992
  and 135,419/s (p99 <=202/164ms). All 8m handled once, zero errors.
  18.6 averaged ~3.2% higher but ranges overlap; no decisive improvement
  or sustainable maximum established. Evidence and comparison.json:
  /private/tmp/vulkan-native18.D1I5CE/; 17 evidence remains under the
  earlier session directory. Runner now defaults to PostgreSQL 18 tools.
  Bottleneck investigation: storage budget is now 100 GB. Scratch runs
  capture 10Hz PostgreSQL session waits, I/O/WAL/checkpoint deltas, actual
  connection-pool waits, allocation profiles and optional mutex/block
  profiles. One-minute automatic baseline: 7,861,095 messages (~131k/s),
  p99 <=198ms. Explicit four-batch callers: 7,797,000 (~130k/s), p99
  <=441ms; kept four database sessions busy instead of ~three and used
  less application CPU, but did not increase throughput. Pool waits and
  GC were small. work_mem=32MB eliminated 2.37GB of consumer-query spills
  but did not improve throughput (~128.5k/s). Raising max_wal_size to 8GB
  and min_wal_size to 2GB regressed to ~94.8k/s, p99 <=4.232s: foreground
  relation writes increased to 4.52GB / 88s cumulative across backends.
  Faster background flushing only recovered ~100.2k/s with p99 <=8.056s;
  reverted WAL limits and background-writer settings, retaining work_mem.
  Eight explicit batch callers reached ~149.2k/s over 30 seconds, but
  p99 rose to <=2.640s; this is not a validated sustainable maximum.
  With the same eight callers and GOMAXPROCS=2 for both application
  processes, the next 30-second run reached ~124.4k/s, p99 <=2.998s.
  This suggests application-parallelism sensitivity, but needs repeated
  comparisons because storage waits also differed between the runs.
  Broaden the investigation beyond database settings: attribute consumer
  cursor/range-lock waits to their blocking transactions; compare multiple
  producer and consumer processes at equal total concurrency; measure Go
  scheduling and per-thread CPU; correlate loopback traffic and socket
  waits with device transfers, throughput and write latency. A single
  consumer cannot rule out contention between same-group consumers.
  Whole-device samples showed ~6.8k–16.6k transfers/s and ~570–1150MB/s;
  these include other host activity and do not prove device saturation.
  All completed runs
  handled every message once with zero application errors; no library
  code changes were made in this investigation. Evidence remains in the
  native18 scratch session directory, with analysis.txt for analyzed runs.
- [ ] Choose retention from measured storage, then validate finalists.
- [ ] Record comparison and sustainable result with evidence.

### 6. Idle-fleet scenario

- [ ] `idlefleet` scenario: N topics and groups sized to 100 / 1k / 10k
  worker rows, one `hold 0/s` phase, consumer replicas 1 / 2 / 3 by
  `--scale`. The observer's `pg_stat_database` and `pg_stat_statements`
  deltas give QPS; `docker stats` on the postgres container gives CPU.
- [ ] Writeup `results/idlefleet/RESULTS.md`; the curve picks the rung on
  the ROADMAP's fix ladder (poll_rate coarsening, idle backoff, LISTEN /
  NOTIFY) and the ROADMAP item is rewritten to the chosen rung.

### 7. Tier 1: debug-buffer overhead

- [ ] `go test -bench` in `pkg/common/logging`: `NewPipelineLogger` with
  `Buffer` on vs off, healthy path (capture, no drain), per operation;
  `-count=10`, benchstat comparison.
- [ ] The number closes the [0559] adoption gate for always-on capture in
  a decision record; no site page.

### 8. Fold-in and close-out

- [ ] `compaction` becomes a keyed scenario (message keys, cardinality,
  producer count) and its driver, `sweep.sh`, `container.sh`, and `env.sh`
  are deleted; `RESULTS.md` and `results/cells.jsonl` stay as history with
  a pointer to the scenario.
- [ ] `fillfactor` harness deleted ([0578] adopted nothing); `RESULTS.md`
  and `cells.jsonl` stay. `trigger_fanout` deleted outright (nothing was
  ever recorded). `scale` stays as history untouched. `idempotency` keeps
  its gitignore rule and `RESULTS.md`. `bench/claim` is a statement
  profile, not a benchmark; it stays.
- [ ] Root `.gitignore`: the stale `/bench/*/driver/driver` and
  `/bench/scale/projector/projector` rules go with their binaries.
- [ ] Citations of the deleted `bench/alertcadence` in HISTORY (2026-09-07
  cadence entry), decision 0709, and `concepts/alert-history.mdx` are
  reworded to state the measurement without the link.
- [ ] `concepts/reliability-lab.mdx` documents the shipped measurement,
  observer, fingerprint, and `runs.jsonl`; the Proposed chaos run stays
  Proposed.
- [ ] Decision records for what this window settled (the lab as the
  harness, guards as checks, the record and rep shape); HISTORY entry;
  the two ROADMAP items removed; root `_bench-design.md` and
  `_bench-methodology.html` deleted.
