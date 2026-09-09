# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
.docs/decisions/.

## Benchmark-recording pipeline on the reliability lab [0687] [0696] [0697]

`.bench/reliability` is the Postgres-bound benchmark harness. A benchmark is
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

Verification per chunk: `cd .bench && go build ./... && go vet ./... && go
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
`golang.org/x/sync` is now a direct import of the `.bench` module; its
`// indirect` marker in .bench/go.mod is stale and left for a hand edit,
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
  .bench/reliability/results/throughput/evidence/20260908T020110Z/.
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
  See .bench/reliability/results/throughput/RESULTS.md.
  Long runs also need bounded recording/import storage; the current
  all-records/all-messages method exceeds 20 GiB before 15m at high rates.
- Native scratch exploration (2026-09-08): reliability suite paused at the
  user's request. Code: .bench/scratchnative/main.go; runner: run.py there.
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
  profiles. The experiment ledger below records changes and verdicts;
  rates are produced counts divided by production elapsed time, with a
  concurrently running consumer. Every run drained completely with zero
  application errors or duplicate handler calls. Drain completion alone
  does not establish sustainable throughput. Latency is end-to-end p99.

  | Run (`scratch_` suffix) | Configuration / change | Window | Rate | p99 upper bound | Outcome / decision |
  | --- | --- | --- | --- | --- | --- |
  | 113643 | Automatic batching, 4096 callers, 4 transactions, batch 1000; shared_buffers 2560MB, work_mem 4MB, max_wal_size 2GB | 60s | 130.9k/s | 198ms | Baseline; ~3 busy producer sessions; pool waits and GC small. |
  | 113912 | Existing ProduceBatch API, 4 callers, batch 1000; same database settings | 60s | 129.9k/s | 441ms | Application CPU fell ~25%, four producer sessions busy; no throughput gain. Consumer query spilled ~2372MiB. |
  | 114206 | Same explicit batches; work_mem 32MB | 60s | 128.5k/s | 491ms | Removed temporary-file spills; no throughput gain. Retained to eliminate this waste. |
  | 114555 | Same; max_wal_size 8GB, min_wal_size 2GB | 60s | 94.8k/s | 4232ms | Regression: foreground relation writes 4.52GB / 88s cumulative backend write time. Fewer checkpoints did not help. |
  | 114815 | Same larger WAL limits; bgwriter_delay 50ms, bgwriter_lru_maxpages 2000 | 60s | 100.2k/s | 8056ms | Still regressed. Reverted WAL limits to 2GB/80MB and background writer to defaults. |
  | 115107 | Restored database settings, work_mem 32MB; automatic 8192 callers / 4 transactions; mutex/block profiling enabled | 30s | 123.3k/s | 800ms | Diagnostic, not a fair speed comparison: extra profiling can perturb execution. ~4 busy producer sessions; native backend stacks show relation extension writes and locks. |
  | 115605 | Explicit 8 callers, batch 1000, GOMAXPROCS 10; restored settings, work_mem 32MB | 30s | 149.2k/s | 2640ms | Higher short-run rate, but latency exceeds the agreed 1s target; not an accepted sustainable result. |
  | 115721 | Same 8 callers; GOMAXPROCS 2 for both application processes | 30s | 124.4k/s | 2998ms | Possible application-parallelism sensitivity; needs alternating repeats because storage waits also differed. |
  | 121859 | Added 10Hz saved cursor/fence state and blocking PID attribution; explicit 4 callers | 30s | 154.3k/s | 155ms | Median saved-settled minus claimed gap 0; no consumer blocking samples. Instrumented probe, not sustained validation. |
  | 122058 | Two producers / two consumers; 20k-message smoke test with disjoint producer identities and consumer bitset union | <1s production | — | 95ms | Cross-process count and duplicate checks passed; not a performance result. |
  | 122144 | Explicit 4 transactions in one producer process; one consumer | 30s | 146.0k/s | 618ms | Matched baseline for process comparison. |
  | 122231 | Same aggregate 4 transactions split across two producer processes | 30s | 141.6k/s | 362ms | No process-scaling benefit; both producers together used ~0.6 CPU cores, consumer ~1.9. |
  | 122319 | Repeat one producer / 4 transactions | 30s | 139.8k/s | 577ms | Brackets the two-producer result; no evidence of a single-producer-process ceiling here. |
  | 122407 | One producer / 8 transactions, one consumer | 30s | 162.5k/s | 2091ms | Higher rate, unacceptable latency. Median saved-settled minus claimed gap ~106k IDs: visibility fence alone does not explain consumer delay. |
  | 122448 | Same aggregate 8 transactions split across two producers | 30s | 122.7k/s | 6274ms | No improvement; heavier foreground write stalls confound attribution to process count alone. |
  | 122529 | One producer / 8 transactions; two consumers, each 2 handlers / queue 8000 (same aggregate handler/queue capacity) | 30s | 107.0k/s | <=3687ms for each consumer | Consumer claim-lock contention confirmed. Blocking sessions were reading messages (86 links) or committing (85); cursor lock spans read and commit. Does not explain the entire rate regression. |
  | 122718 | Repeat one producer / 8 transactions, one consumer | 30s | 139.6k/s | 2978ms | Substantial run variation persists; latency still fails. |
  | 122801 | Attempt Unix socket through NewPostgresPool host parameter | No production | — | — | Setup failed: URL host rejects socket-directory escape. Setup-only DB removed; scratch switched to supported caller-supplied pgx pool. No library change. |
  | 122934 | Unix socket using caller-supplied pgx pool; 10k-message smoke | <1s production | — | 95ms | Passed; transport comparison can proceed without library changes. |
  | 123030 | TCP, 8 transactions; caller-supplied pgx pool | 30s | 160.5k/s | 1648ms | New-binary baseline; latency fails. |
  | 123110 | Same settings via Unix socket | 30s | 147.1k/s | 5037ms | No demonstrated improvement; rate falls inside surrounding TCP results. |
  | 123151 | Repeat TCP | 30s | 120.7k/s | 3857ms | Wide variation makes transport comparison inconclusive. |
  | 123229 | Repeat 2 consumers / aggregate 4 handlers and queue 16000, 8 producer transactions | 30s | 116.8k/s | <=3353ms for each consumer | Frequent fresh-claim lock waits reproduced (146 samples after warmup); no scaling win demonstrated. |
  | 123309 | One producer / 16 transactions, one consumer | 20s | 134.8k/s | >=10s (histogram overflow) | No gain. ~15 non-idle producer sessions but only ~3.7 running; most sampled waits involved buffer content, relation extension or WAL. Median saved-settled minus claimed gap 733k IDs. |

  | 123521 | Delayed-consumer startup gate, 10k-message smoke | <1s production | — | Not applicable | Passed; consumer starts only after producer stops, then counts/identities verified. |
  | 123529 | Explicit 8 transactions, concurrent consumer; completed checkpoint before production | 20s | 130.5k/s | 2731ms | Baseline for consumer-load isolation; latency fails. |
  | 123556 | Same producer settings; start consumer only after production finishes | 20s production | 133.0k/s | Not applicable | Diagnostic only: intentional backlog, not sustainable throughput. Removing concurrent consumer/manager activity gave no large producer-rate gain. All 2.664m subsequently consumed once. |
  | 123639 | Repeat concurrent consumer, checkpoint before production | 20s | 125.4k/s | 1754ms | Brackets deferred-consumption result; database write path remains the stronger throughput suspect at this configuration. |

  | 124529 | Checkpointed 8-transaction baseline, shared_buffers 2560MB | 60s | 138.4k/s | 2548ms | Foreground relation writes ~798MB / 19.25s cumulative; reference for 6GB buffer comparison. |
  | 124726 | Same 8 transactions; shared_buffers increased to 6GB, native restart | 60s | 137.3k/s | 2347ms | Foreground relation writes fell from ~798MB/19.25s to ~90MB/2.30s; throughput unchanged. Foreground eviction writes alone do not explain the ceiling. Restored 2560MB for repeat. |
  | 124909 | Restore shared_buffers 2560MB and restart; matched repeat | 60s | 119.9k/s | 5927ms | Large run variation persists; no demonstrated throughput win from 6GB. |
  | 125028 | Two consumers, each claim 4000 / 2 handlers / queue 8000; producer batch 1000, 8 transactions | 30s | 121.2k/s | <=7363ms per consumer | Reference for shorter consumer claims. |
  | 125109 | Same two consumers; claim 1000 each | 30s | 120.3k/s | <=2281ms per consumer | Similar rate, lower latency tail in this pair; frequent claim-lock waits remain. |
  | 125221 | One consumer, claim 4000; producer batch 250, 8 transactions | 30s | 121.2k/s | 267ms | Shorter producer transactions sharply reduced tail latency and saved-settled/claimed gap. Candidate for longer observation. |
  | 125302 | Repeat producer batch 1000 / 8 transactions | 30s | 93.4k/s | 2364ms | Worse paired control, but run variation prevents attributing the whole difference to batch size. Live EXPLAIN showed sequential cleanup selection despite an existing created_at index. |

  | 125510 | Producer batch 250 / 8 transactions, default analysis timing; table-statistics monitoring | 120s | 84.9k/s | 1259ms | Short-run latency gain did not establish sustainability. Idempotency auto-analysis first observed near 118s; message partitions/parent remained unanalyzed. Cleanup spent 12.319s deleting zero rows. |
  | 125849 | Same workload; explicit ANALYZE of message parent and idempotency table every 15s after previous analysis completes | 120s | 84.5k/s | >=10s (histogram overflow) | Cleanup selection changed to index scan; total cleanup time fell to 0.152s. No throughput gain. ANALYZE calls took 1.63s, 27.88s, 31.07s; prolonged saved-fence stalls coincided with maintenance. Rejected as a usable schedule; opt-in only, default remains off. |

  | 130509 | Two-producer/two-consumer 20k-message smoke; expanded all-session sampler | <1s production | — | <=109ms per consumer | Counts/bitset union passed; 13 sampler frames validated, including database and backend type. |

  Current native settings: PostgreSQL 18.6, shared_buffers 2560MB,
  work_mem 32MB, max_wal_size 2GB, min_wal_size 80MB, default background
  writer; durable settings remain enabled. No sustainable maximum is
  established. Evidence for each row is under
  `/private/tmp/vulkan-native18.D1I5CE/scratch_<suffix>/`: command.json,
  binary.sha256, runtime configuration logs, settings, per-second counts,
  waits, profiles and verification.txt. Completed raw artifacts are now retained locally under
  `.bench/scratchnative/results/evidence/native18/` (git-ignored), with
  symlinks from the original temporary paths. This ledger is the tracked
  findings record; raw files still need separate backup if the workspace
  is deleted. Each run records its executable hash and resolved settings.
  Process-scaling and transport checks above used the same database
  settings, with no checkpoint forced before each run. Later isolation
  tests explicitly finish a CHECKPOINT before measurement to reduce
  variation in the starting flush state; their command.json records it.
  Native thread samples found ~14–15 additional threads per application
  process. Both process-count tests and CPU attribution weaken the
  single-producer-process hypothesis at four transactions; they do not
  rule out consumer dispatch/decode limits at higher rates. Two-consumer
  claim-lock contention is reproduced, but physical SSD saturation is
  still unproven. Unix sockets have no demonstrated advantage.
  Additional finding: the 6GB run's idempotency cleanup spent ~3.39s
  across 12 calls deleting zero rows. The created_at index exists in the
  running schema; a live EXPLAIN of the cutoff selection nevertheless
  chose a sequential scan. Investigate statistics/plan selection before
  calling this a missing-index problem. New samples record table-analysis
  timestamps and modified-row counts; new runs also hash the runner.
  Longer-run finding: improving statistics removed the expensive empty
  cleanup scans but did not improve overall throughput. During maintenance,
  saved-settled/claimed positions stopped while visible production advanced;
  the snapshot xmin did not match a sampled producer/consumer transaction.
  This is consistent with an external maintenance transaction holding the
  global visibility fence, not conclusive attribution: the full-session
  query missed the active window. The sampler now includes maintenance
  sessions and transactions with assigned xids in other native databases.
  Partition-creation relation-lock waits were also captured in
  maintenance-blockers.json; neither explains the whole stall yet.
  PostgreSQL does not automatically analyze partitioned parents:
  https://www.postgresql.org/docs/18/routine-vacuuming.html . A representative
  warmup/analysis policy and maintenance/fence interaction need investigation
  before longer sustainable-throughput claims.
  Continuation totals: 58,054,250 additional messages across 10 runs,
  matching production/consumption counts with zero application errors or
  duplicate handler calls. Run index now has 41 records; local raw evidence
  is ~176MiB. Databases were cleaned up, shared_buffers restored to 2560MB,
  work_mem remains 32MB, and periodic ANALYZE is disabled by default.
  Remaining: distinguish PostgreSQL hot-page/extension/WAL serialization
  from the physical device's sustained write limit; measure consumer
  claim/read/dispatch stages more finely before proposing library changes.
  Producer processes and native threads have been checked, and matching
  consumer-load isolation did not expose a large producer-rate increase.
  Same-group consumer lock contention is confirmed, not merely suspected;
  claim-blockers.json contains timestamped waiting/blocking PID examples.
  No configuration from this pass meets a long-duration sustainable
  maximum verdict. The original 1 KB / one-group target remains in force.
  This pass produced and subsequently consumed 60,394,000 messages with
  matching counts and zero application errors/duplicate handler calls.
  One socket-setup failure occurred before production and is recorded.
  `.bench/scratchnative/results/runs.jsonl` indexes retained runs across
  the native18 session; successful new runs append automatically and move
  their evidence into the workspace (original paths remain symlinks). Disposable databases
  and benchmark sessions were removed; the native PostgreSQL service stays
  running. Scratch build/vet and cross-process/start-gate/socket smoke
  checks passed; `go test -race ./scratchnative` compiled successfully but
  the scratch package contains no test files.
  Whole-device samples showed ~6.8k–16.6k transfers/s and ~570–1150MB/s;
  these include other host activity and do not prove device saturation.
  All completed runs
  handled every message once with zero application errors; no library
  code changes were made in this investigation. Evidence remains in the
  native18 scratch session directory, with analysis.txt for analyzed runs.
- Producer-only isolation (2026-09-08, explicitly requested): run.py now
  supports CONSUMERS=0. No consumer/manager process is launched, even for
  setup; worker_instance is asserted empty before and after production.
  These runs measure production only and are not produce-and-consume
  throughput claims. Payload remains 1 KB, orders is one topic, PostgreSQL
  18.6 remains native/durable; initial shared_buffers 2560MB, work_mem 32MB.
  Sweep batch size, transaction concurrency, process count and Go parallelism;
  follow short probes with repeated longer runs. Evidence and settings append
  automatically to the existing run index. 131609 validated producer-only
  mode with 10k messages; 131702 (one transaction, batch 1000) reached
  76.4k/s over 10s; 131714 (two transactions) reached 127.2k/s over 10s.
  With 1000-message explicit batches, additional 10s probes were 131735:
  4 transactions / 183.0k/s; 131758: 8 / 194.5k/s; 131817: 16 / 194.9k/s;
  131844: 32 / 189.9k/s. These are short production-only rates, not a
  sustained maximum. No failures and database counts matched every run.
  Batch-size sweep at eight transactions (10s each): 132001 batch 100 /
  140.5k/s; 132022 batch 250 / 179.2k/s; 132045 batch 4000 / 148.0k/s;
  132111 batch 16000 / 64.0k/s. Multi-process probes with batch 1000:
  132123 two processes × four transactions / 181.1k/s; 132155 four × two /
  191.9k/s; 132221 four × eight / 165.1k/s. No gain over one process with
  8–16 transactions; very large batches regressed sharply.
  Later 10s runs at eight transactions / batch 1000: 132320 GOMAXPROCS=1
  152.2k/s; 132347 GOMAXPROCS=2 151.1k/s; 132420 GOMAXPROCS=10 with CPU
  profiling disabled 151.9k/s. These clustered below the early ~195k/s,
  so they do not establish causal rankings for Go parallelism or profiling.
  Host inspection also found active macOS Storage/StorageManagementService
  processes; physical-device and host-state variation remain confounders.
  Larger PostgreSQL budget (shared_buffers 6GB, max_wal_size 8GB,
  min_wal_size 2GB; durability unchanged): 132556 explicit batch 1000 /
  8 transactions reached 208.9k/s over 15s; 132614 automatic batching /
  16384 callers / 8 transactions reached 187.7k/s over 15s. Full-minute
  runs with CPU profiling disabled: 132735 eight transactions produced
  9,404,000 in 60.412s (155.7k/s, produce-call p99 <=164ms); 132933 sixteen
  produced 8,586,000 in 60.096s (142.9k/s, p99 <=419ms). Counts matched,
  errors were zero, consumers were absent. First/last ten sample intervals:
  eight transactions 203.5k/150.1k per second; sixteen 199.9k/85.0k.
  Increasing concurrency raised mean PostgreSQL CPU from 373% to 475% of
  one core, BufferContent wait samples from 418 to 2626, and foreground
  relation writes from 1.36GB to 2.07GB. Producer CPU stayed about 72% of
  one core and pool acquisition waits were negligible. This supports
  database-side contention/write pressure, not a proven physical SSD or
  absolute PostgreSQL ceiling. The 208.9k/s result remains a short burst.
  Repeat 133446 (eight transactions, same settings) produced 8,591,000
  in 60.019s (143.1k/s, produce-call p99 <=162ms), with zero errors and
  matching database count. Thus the two eight-transaction minute averages
  span 143.1–155.7k/s; no sustained maximum or 300k/s reproduction is
  established. Current native PG settings remain shared_buffers 6GB,
  max_wal_size 8GB, min_wal_size 2GB, work_mem 32MB, durable commits on.
  Final repeat independently verified 8,591,000 distinct stored payload
  sequence IDs spanning 1–8,591,000 after measurement; validation is excluded
  from throughput. No scratch databases or benchmark connections remain.
  Retained native clusters total about 10GiB plus 206MiB evidence; host has
  149GiB free. All observed storage remained below the 100GB allowance.
  Python syntax checks and git diff --check passed; nothing committed.
- pgxpool isolation (2026-09-08): exposed the scratch binary's existing
  -connections flag through POOL_CONNECTIONS in run.py; binary unchanged.
  Prior winning eight-transaction runs had MaxConns=32, peak acquired=8,
  peak total=9 and only 0.046–0.048s cumulative empty-pool acquisition wait
  over a minute (including connection construction). Pool capacity was not
  restricting those runs. Producer-only 10s comparisons, batch 1000, same
  native durable PG settings, CPU profiling off: 134127 64 callers / pool32
  163.2k/s; 134140 64 callers / pool96 147.3k/s. Acquired connections rose
  32 -> 64 and ongoing empty-pool waits disappeared; BufferContent samples
  rose 1158 -> 2693 over comparable post-warmup windows. Eight-caller
  controls: 134152 pool96 208.4k/s; 134204 pool32 215.8k/s. Both acquired
  eight connections with zero post-warmup empty-pool wait. The latter is a
  new best 10s burst, not a replacement for the recorded minute results.
  Reverse-order repeats: 134321 64 callers / pool96 152.5k/s; 134334
  64 callers / pool32 170.1k/s. Both comparisons favored the smaller pool
  by about 10%; the larger pool removed application waiting but increased
  database contention. All six runs had zero errors, matching database
  counts and no workers. Scratch databases were dropped; MaxConns defaults
  to 32 unless explicitly overridden. Other pool lifecycle settings were
  not changed: no evidence of sustained connection acquisition pressure at
  the best eight-transaction configuration. No library changes or commits.
- Lower pool-limit follow-up (2026-09-08): kept 64 concurrent ProduceBatch
  callers, batches of 1000, one process and all other settings fixed; ran
  pool limits 8,16,32,64 then 64,32,16,8, ten seconds each. Results in k/s:
  pool8 193.2 / 205.6 (134918 / 135043); pool16 208.6 / 203.3 (134931 /
  135030); pool32 177.8 / 177.4 (134943 / 135018); pool64 126.2 / 144.2
  (134955 / 135007). Reducing active DB concurrency below 32 helped here;
  8 versus 16 is not conclusively ranked by these two short repetitions.
  Per-run pool-analysis.json records a common production-time window from
  second 3 through second 9 (60 PostgreSQL samples), excluding startup and
  drain. Mean BufferContent-waiting sessions: pool8 0.82–1.17; pool16
  6.23–6.82; pool32 20.98–21.57; pool64 48.20–51.28. Active sessions with
  no reported wait remained about 4–6, not proportional to connection count.
  pg_stat_statements execution time per protected INSERT, entire run:
  pool8 0.023–0.026ms; pool16 0.060–0.061ms; pool32 0.155–0.159ms;
  pool64 0.405–0.460ms. These elapsed execution times include server-side
  waiting, not just CPU, and do not include time waiting to acquire the pool.
  Mechanism supported: Pool.Begin acquires a connection for the transaction
  through SendBatch and Commit; limiting connections queues excess callers
  before database work, reducing simultaneously competing INSERTs. PostgreSQL
  defines BufferContent as waiting to access a data page in memory; exact
  contested relation/page and CPU scheduling contributions remain unproven.
  Sources: https://www.postgresql.org/docs/18/monitoring-stats.html and
  https://wiki.postgresql.org/wiki/Number_Of_Database_Connections .
  All eight runs verified matching counts, zero errors, no consumer workers;
  scratch databases dropped. These establish a short-run concurrency effect,
  not a long-run optimum. Pool default remains 32; no code changed this turn.
- Expanded settings sweep (2026-09-08, user authorized all proposed areas):
  scratch-only flags now expose partition size, pgx execution mode and cache
  capacity; resolved pool settings and actual topic settings are retained.
  Rebuilt binary used consistently for this sweep; smoke 140023 verified 10k
  messages, no workers. Targeted benchmark-module fmt/build/vet/race passed
  (no test files). Planned sequence: joint batch/caller/pool region; Go thread
  and GC/memory settings crossed with batch size; partition boundaries;
  query mode/cache/Unix transport; PostgreSQL buffers/background writes/WAL
  and checkpoint pacing; longer finalist confirmations. Durability and 1KB
  payload stay fixed. Existing run index retains every completed experiment.
- Expanded joint sweep results (10s each, new binary, no errors/count mismatches):
  140043 callers=8, pool=32, batch=500: 196.5k/s.
  140055 callers=8, pool=32, batch=1000: 225.1k/s.
  140107 callers=8, pool=32, batch=2000: 222.4k/s.
  140119 callers=16, pool=32, batch=500: 209.5k/s.
  140131 callers=16, pool=32, batch=1000: 207.2k/s.
  140143 callers=16, pool=32, batch=2000: 184.9k/s.
  140155 callers=64, pool=16, batch=500: 207.2k/s.
  140207 callers=64, pool=16, batch=1000: 205.4k/s.
  140219 callers=64, pool=16, batch=2000: 133.0k/s.
  Eight callers / pool32 / batch1000 is the control for subsequent sweeps.
- Runtime and pgx sweep (10s, eight callers/pool32; resolved configs retained):
  140255 batch=1000, threads=2, GOGC=400, memory=2GiB, mode=cache_statement, cache=512, host=127.0.0.1: 223.0k/s.
  140308 batch=1000, threads=4, GOGC=400, memory=2GiB, mode=cache_statement, cache=512, host=127.0.0.1: 216.2k/s.
  140320 batch=1000, threads=10, GOGC=100, memory=2GiB, mode=cache_statement, cache=512, host=127.0.0.1: 200.4k/s.
  140332 batch=1000, threads=10, GOGC=400, memory=256MiB, mode=cache_statement, cache=512, host=127.0.0.1: 211.6k/s.
  140344 batch=1000, threads=10, GOGC=400, memory=2GiB, mode=cache_statement, cache=512, host=127.0.0.1: 199.8k/s.
  140356 batch=2000, threads=2, GOGC=400, memory=2GiB, mode=cache_statement, cache=512, host=127.0.0.1: 211.1k/s.
  140408 batch=2000, threads=4, GOGC=400, memory=2GiB, mode=cache_statement, cache=512, host=127.0.0.1: 203.4k/s.
  140420 batch=2000, threads=10, GOGC=100, memory=2GiB, mode=cache_statement, cache=512, host=127.0.0.1: 194.0k/s.
  140432 batch=2000, threads=10, GOGC=400, memory=256MiB, mode=cache_statement, cache=512, host=127.0.0.1: 207.4k/s.
  140444 batch=2000, threads=10, GOGC=400, memory=2GiB, mode=cache_statement, cache=512, host=127.0.0.1: 203.4k/s.
  140456 batch=1000, threads=10, GOGC=400, memory=2GiB, mode=exec, cache=512, host=127.0.0.1: 107.2k/s.
  140508 batch=1000, threads=10, GOGC=400, memory=2GiB, mode=cache_statement, cache=32, host=127.0.0.1: 203.0k/s.
  140520 batch=1000, threads=10, GOGC=400, memory=2GiB, mode=cache_statement, cache=512, host=/tmp: 193.4k/s.
  140532 batch=1000, threads=10, GOGC=400, memory=2GiB, mode=cache_statement, cache=512, host=127.0.0.1: 201.2k/s.
  Exec mode sharply regressed; other differences do not establish repeatable
  improvements over controls. Keep threads10/GOGC400/memory2GiB, prepared
  cache512 and TCP for subsequent comparisons. All counts/errors checked.
- Partition sweep (eight callers, batch1000, pool32):
  140559 partition=100000: 192.9k/s over 10.017s.
  140611 partition=500000: 191.6k/s over 10.017s.
  140622 partition=5000000: 111.6k/s over 60.103s.
  140750 partition=20000000: 118.9k/s over 60.023s.
  Small-partition SQL profiles confirm producer-side CREATE TABLE work;
  both minute runs still declined. Host snapshot during post-run checks
  found OrbStack Helper at 266.5% of one CPU core, StorageManagementService
  at 31.9%, fseventsd at 30.5%; these are confounders, not attributed Vulkan
  work. Added host_processes to subsequent monitor rows using the existing
  ps sample, with no extra sampling process. Keep partition20m for PG
  sweeps to exclude partition crossings; all other app settings unchanged.
  PostgreSQL test order: bgwriter maxpages1000; maxpages1000/delay50ms;
  shared_buffers3GB; shared_buffers9GB; max_wal_size2GB; checkpoint target0.5.
  Each starts from the 6GB/8GB/100pages/200ms/0.9 baseline and restarts only
  the dedicated cluster. All are 60s, durability remains enabled; restore
  baseline automatically after these six runs. Raw settings capture each.
- First PostgreSQL sweep results: 140944 maxpages1000/delay200ms produced
  7,569,000 in 60.809s (124.5k/s), foreground relation writes 0.135GB;
  141155 maxpages1000/delay50ms produced 7,431,000 in 60.050s (123.7k/s),
  foreground writes 0.555GB. Neither completed a requested checkpoint
  during measurement. Post-warmup WALWrite LWLock samples were 1388/1175,
  versus DataFileWrite samples 78/391. Moving some relation writes off
  producer backends did not establish a throughput improvement. Exact
  device limits remain unproven; host-process snapshots are retained.
- Completed initial PG cases, in the test order recorded above:
  140944: 124.5k/s; requested/done checkpoints 0/0; foreground writes 0.135GB.
  141155: 123.7k/s; requested/done checkpoints 0/0; foreground writes 0.555GB.
  141404: 101.8k/s; requested/done checkpoints 2/1; foreground writes 3.154GB.
  141621: 121.8k/s; requested/done checkpoints 0/0; foreground writes 0.000GB.
  141738: 101.3k/s; requested/done checkpoints 2/2; foreground writes 0.286GB.
  141923: 113.5k/s; requested/done checkpoints 0/0; foreground writes 0.656GB.
  Nine GB buffers eliminated measured relation writes but retained heavy
  WALWrite waiting and did not establish a throughput gain. The 0.5-target
  case performed no checkpoint writes, so it is not evidence about pacing;
  follow with max_wal_size2GB/target0.5 against the 2GB/0.9 case that did
  checkpoint. Then restore all baseline settings and run batch2000/batch1000
  minute confirmations. No errors or count mismatches in these six cases.
- Active-checkpoint follow-up 142147: max_wal_size2GB and target0.5,
  otherwise baseline, produced 8,856,000 in 60.029s (147.5k/s), with four
  requested/completed checkpoints, 51.556s checkpoint write time, 0.685s
  sync time and 0.027GB foreground relation writes. Compare 141738 at
  target0.9: 101.3k/s, two completed checkpoints and 5.041s sync time.
  This is a candidate interaction, not a causal improvement established by
  repeats: mean sampled non-benchmark ps CPU was 69.6% versus 117.6% of one
  core, and I/O/host state may differ. Restored max_wal_size8GB/target0.9.
  Baseline batch2000 confirmation 142339: 8,212,000 in 60.032s (136.8k/s),
  produce-call p99 <=1064ms. Batch1000 control 142534 produced 6,386,000
  in 60.063s (106.3k/s, p99 <=936ms); the spread warranted a repeat.
- Checkpoint candidate repeat 142723 (2GB WAL/target0.5/batch1000):
  10,785,000 in 60.016s (179.7k/s), p99 <=113ms, four checkpoints completed,
  zero errors and matching DB count. This exceeds prior minute results;
  the same candidate previously reached 147.5k/s, so a fixed optimum is
  not established. Test batch2000 with the same checkpoint settings, then
  give the stronger candidate a longer confirmation with a nearby control.
- Checkpoint/batch interaction 142951: same 2GB WAL/target0.5 with batch2000
  produced 9,242,000 in 60.032s (154.0k/s), p99 <=655ms. Batch1000 remains
  the stronger measured candidate. Final confirmation sequence: nearby
  8GB WAL/target0.9 baseline minute, then 2GB WAL/target0.5 for 90s with
  batch1000, independent distinct stored sequence verification afterward.
- Nearby baseline 143324 (6GB buffers, 8GB WAL, target0.9, batch1000,
  eight callers/pool32, partition20m) produced 11,152,000 in 60.031s
  (185.8k/s, p99 <=85ms), exceeding the checkpoint candidate repeat.
  One checkpoint completed. This prevents attribution of the improved
  rates to checkpoint tuning. Normal WAL writes: 15.791GB in 6.391s
  cumulative backend write time, versus 11.209GB in 27.382s in 141621.
  These runs differ in buffer setting too (6GB versus 9GB), and may differ
  in host/filesystem/checkpoint state; the measured write cost is materially
  variable, not evidence of a fixed device throughput ceiling. The identical
  app/PG baseline 142534 was 106.3k/s earlier. Preserve the full spread;
  do not present the best minute as a repeatable configuration improvement.
- Longer candidate confirmation 143605: 2GB WAL/target0.5, batch1000,
  eight callers/pool32, partition20m produced 11,950,000 in 90.121s
  (132.6k/s, p99 <=477ms), five completed checkpoints. Mean rates across
  the first/middle/last 30 one-second samples were 181.3k/119.1k/97.7k/s.
  Thus the checkpoint candidate did not sustain its best minute rate and
  has not beaten baseline repeatably. Restore 6GB shared_buffers, 8GB WAL,
  min WAL2GB, target0.9, bgwriter100pages/200ms; retain application defaults
  for scratch control (8 explicit batch callers, batch1000, pool32,
  threads10/GOGC400/memory2GiB, cached statements512, TCP). Final identity
  verification and cleanup follow outside the measurement interval.
- Expanded sweep close-out: 40 measured runs plus one smoke, 171,787,500
  total verified produced rows; every run had zero errors, matched DB count,
  empty consumer result and producer-only worker assertions. Final 143605
  additionally verified 11,950,000 distinct sequences spanning 1–11,950,000.
  All scratch databases/connections removed. Native18 baseline restored and
  checked: shared_buffers6GB, WAL8GB/min2GB, checkpoint target0.9,
  bgwriter100pages/200ms; fsync/synchronous_commit/full_page_writes/checksums/
  autovacuum all on. Retained clusters about 4GiB plus evidence; host150GiB
  free, no storage-guard breaches. Targeted bench fmt/build/vet/race-build
  passed (no test files); Python AST and git diff --check passed. No commits.
  Best measured new-binary rates: 225.1k/s over 10s (140055), 185.8k/s over
  60s (143324); longer checkpoint candidate 132.6k/s over 90s (143605).
  Keep these durations distinct. No repeatable sustained configuration
  improvement or absolute maximum established by the expanded sweep.
- RCA isolation (2026-09-08, user approved the three diagnostic comparisons):
  added optional per-second PostgreSQL IO/WAL/checkpointer snapshots and
  native iostat output (first since-boot row excluded), aligned by wall time
  with producer and 10Hz wait samples. A direct pgx branch uses the final,
  non-compacted protectedInsertSQL fmt template extracted from current
  production source; frozen source/template hashes are checked and copied
  into evidence. It retains UUIDv7, JSON encoding, six parameter values,
  SendBatch, per-message returned IDs, and durable transaction commits;
  it omits Vulkan validation/adapters/result construction/retry wrappers.
  No partition crossing (20m), no failures, no consumers. Semantic smoke
  runs 145446 Vulkan / 145448 pgx both produced 10k rows and UUIDv7 keys,
  nil optional fields/schema version1, exactly ten 1000-message transactions;
  pg_stat_statements normalized INSERT query text matched byte-for-byte.
  Full semantic scans are opt-in thereafter; existing row/batch checks stay.
  Added a 512MiB bounded file probe: 1MiB pwrite calls with O_DSYNC, target
  10 writes/s, random buffer, initialization/fsync outside timed interval,
  signal-safe normal shutdown/file removal. This mirrors the database's
  open_datasync mechanism, not a stronger hardware durability guarantee.
  It measures latency under light added load, not peak device bandwidth.
  Planned order: probe alone; Vulkan60s; pgx60s twice; Vulkan60s with probe;
  Vulkan60s without probe; probe alone again. Keep PG and app knobs fixed.
  Bench fmt/build/vet/race-build passed; scratch controls are not library
  changes. Existing unrelated README whitespace is left untouched.
- RCA first transitions: standalone O_DSYNC probe (rca_durable/alone-before)
  median of one-second latency medians 1.404ms, largest observed write
  1.848ms at target10MiB/s. Vulkan145710 produced 10,086,000/60.030s
  (168.0k/s). First two ten-second windows 222.8k/209.9k then 145.0k;
  WAL write time stayed 0.51–0.64ms/write over these windows, while
  foreground relation writes changed from zero to 247MB/9.019s cumulative
  write time and file-extension time rose from 2.49s to 4.80s. This
  transition does NOT support WAL latency as the sole slowdown cause.
  Direct pgx150004 produced 9,656,000/60.814s (158.8k/s), showing a similar
  fall from 219.5k/205.0k initially. A later 88.8k/s window coincided with
  checkpoint writing; its final WAL write latency also rose to4.30ms.
  These are different delay modes. Per-ten-second evidence is retained
  in rca-analysis.json; timestamps/counter lag limit sub-second attribution.
- Direct pgx repeat150302 produced 8,180,000/60.029s (136.3k/s):
  initial224.2k/211.0k per ten-second window, late66.9k/90.4k/97.9k.
  WAL write latency rose from1.16–1.51ms to12.25–13.95ms and WALWrite
  waiting averaged2.88–4.16 sessions in the worst windows. The producer
  stack bypass reproduces the slowdown; no application-level speedup was
  demonstrated. Do not conflate this WAL-dominated run with145710, whose
  first slowdown accompanied relation writes with flat WAL latency.
  Added optional BUFFER_SUMMARY (pg_buffercache_summary every5s) for a
  subsequent matched6GB/9GB buffer-budget intervention: test whether the
  onset of foreground writes follows cache filling and moves with its
  capacity, rather than merely ranking whole-run rates. No cache eviction
  functions or durability changes. Current queued comparisons leave it off.
- Independent write-latency control: Vulkan+probe150526 produced7,890,000
  in60.050s (131.4k/s). Probe1MiB O_DSYNC writes had no >20ms sample in
  the first10s; later ten-second-window maxima677/577/277/932/543ms.
  Worst producer window40–50s was78.3k/s, WAL11.42ms/write,3.15 sessions
  waiting WALWrite, and3.254GB checkpoint writes. Typical probe writes
  could still be sub-ms; this is a long-tail completion-latency problem.
  Subsequent Vulkan WITHOUT probe150739 was114.4k/s and reproduced the
  collapse227.2k initially to56.9–98.4k later. The probe is not necessary
  to produce the slowdown, though its incremental impact is not measured
  precisely. Standalone probe after: median of interval medians1.240ms,
  largest write2.264ms (before1.404/1.848ms). Probe files were removed.
  This locates load-dependent completion stalls outside PostgreSQL as
  well as inside it; not a proof of physical SSD bandwidth exhaustion or
  a specific APFS/device/OS mechanism. The probe is a small-file overwrite
  workload; PostgreSQL also extends files and initializes WAL segments.
  Follow with paired6GB/9GB buffer-summary + vm_stat runs,75s each, to
  distinguish cache-filling triggers from stalls while cache is still free.
- Cache-occupancy intervention completed: 6GB151104 produced14,032,000
  in75.016s (187.1k/s); 9GB151419 produced10,937,000 in76.121s
  (143.7k/s), both zero errors, verified rows and no consumers. Cache
  became full between22–28s at6GB and39–44s at9GB. The6GB run still
  delivered160–179k/s after filling, with WAL writes about0.8–1.5ms.
  The9GB run fell to118.1k/s during30–40s while unused buffers remained;
  its40–50s window reached74.5k/s,12.74ms/WAL write and3.53 sessions
  waiting WALWrite. Cache filling alone does not explain the collapses;
  one sequential pair cannot establish that9GB causally reduces throughput.
  Host vm_stat (16KiB pages) recorded zero additional Swapouts in BOTH
  runs. Compressor occupancy grew about1.46GiB at6GB but shrank0.16GiB
  at9GB. Pageouts are separate from Swapouts; these host-wide counters
  do not establish swapping or compression as the cause of the slower run.
- RCA conclusion for this producer-only phase: bypassing Vulkan's producer
  stack does not remove the collapse; independent durable file writes also
  stall under the load. Strongest location is storage-stack write completion,
  with PostgreSQL WALWrite serialization propagating stalls to producers.
  Data-file writes/extensions contribute in runs without rising WAL latency.
  Physical SSD limits vs filesystem/OS writeback vs competing host activity
  remain unseparated. No proof of absolute hardware saturation or sustained
  end-to-end throughput. Next causal fence: repeat a bounded storage-only
  workload matching WAL append plus data-file writeback, with host process
  I/O attribution; compare WAL-only and combined writes. A second physical
  device would be a stronger separation if available, not assumed available.
  Completed artifacts: per-run rca-analysis.json and analysis.txt, raw
  counters/waits/iostat, optional vm_stat/buffer_summary, runs.jsonl index.
  Restored and verified6GB buffers,8GB WAL,target0.9 and durability ON;
  scratch databases removed. Retained native18+old17 clusters about10.8GB,
  host free158GB; last pair peak native18 allocation27.3GB, below100GB.
- Storage-only follow-up (2026-09-08): storage_isolate.py compares a
  single1MiB O_DSYNC sequential writer targeted at250MiB/s against the
  same writer plus8KiB buffered data writes targeted at300MiB/s, fsync
  every second. Two alternating45s pairs; WAL file capped16GiB, data8GiB,
  then sequential reuse, automatic unlink and40GiB host-free guard.
  This approximates write components, not PostgreSQL's full write pattern.
  Three-second smoke reached both target rates and independently observed
  both writer PIDs' disk-byte counters increasing. macOS proc_pid_rusage v2
  layout checked against local SDK sys/resource.h;193 processes were
  inaccessible in smoke, so host attribution is explicitly incomplete.
  No database configuration changes or producers/consumers in this control.
- Storage-only results: wal154305/154437 each held250MiB/s for45s;
  maximum sampled O_DSYNC write6.98/5.97ms. Mixed154351/154523 kept
  WAL250MiB/s, maxima4.89/22.39ms (zero/one intervals over20ms), while
  data averaged242.0/239.6MiB/s after reusing its8GiB file. Data fsync
  maxima51.57/54.64ms. A targeted16GiB data-file control154623 avoided
  reuse and achieved WAL250 + data300MiB/s for45s, write maxima9.29/
  9.01ms, data fsync maximum4.23ms. No earlier hundreds-of-ms WAL stall
  reproduced. These are achieved component rates under fixed targets,
  not measured device maxima or equivalent PostgreSQL workloads.
  Process counters attributed2.75/2.64GB reads to the reused data-file
  writer, versus0.25MB to the growing-file writer. File reuse changes
  storage behavior substantially; the precise read mechanism is unproven.
  Accessible other processes had small I/O relative to test writers;
 193–210 processes were inaccessible, so external I/O is not ruled out.
  iostat host disk rates about250 alone and546–551 combined (tool units);
  first since-boot row excluded. Source hash, per-second latencies, fsync,
  process I/O, vm_stat, disk samples, analysis.json and run index retained.
  The synthetic test uses1MiB WAL writes at250MiB/s; observed PostgreSQL
  early WAL averages were1.4–1.7MiB/write and282–295MiB/s, and PostgreSQL
  additionally updates indexes/pages, initializes segments, and checkpoints.
  Conclusion: aggregate write volume at this tested level is insufficient
  to reproduce the collapse; do not promote storage-stall evidence into
  a claim of physical SSD saturation. Next comparison should retain real
  PostgreSQL workload while attributing reads/writes per backend and
  separating WAL initialization, relation extension, and existing-page
  writes around slowdown onset. The new file-reuse read signal is a lead,
  not an established explanation for PostgreSQL's stalls.
  All five controls completed and removed temporary data files; no PG
  settings changed. Python syntax and live3s smoke verified the new scripts.
- Backend attribution follow-up: BACKEND_IO=1 records PostgreSQL18
  pg_stat_get_backend_io for live PIDs, table/index block reads, and macOS
  per-process disk bytes and CPU time in each monitor sample. Background
  checkpointer has no per-backend PG I/O rows, so use global PG checkpointer
  counters plus its OS PID. Small smoke155807 verified observer fields and
  cleanup; observer's added snapshot took about0.06s in smoke,0.09s during
  initial measured run. SQL/OS counters have different semantics and lag;
  backend-analysis.json assigns intervals by ending sample, omitting exited
  processes and first/last partial intervals. No consumers or settings changes.
- Backend attribution results:155827 produced11,319,000/60.022s
  (188.6k/s),160111 produced9,505,000/60.089s (158.2k/s), errors0.
  First run ten-second rates211/212/181/184/172/172k; second204/208/
  165/113/126/135k. WAL write latency stayed0.6–0.8ms in first and
  about1.0–1.9ms in second; neither reproduced the earlier10ms+ WAL
  collapse. Both caches filled around24–29s. In40–50s producer data-page
  writes rose from156MB/5.9 cumulative seconds in first to1.39GB/28.0s
  in second. Those times sum across eight producer backends; not wall time.
  Producer relation reads were almost absent (first24KiB, second16KiB
  over sampled intervals), and index block reads were negligible. This
  disfavors index-cache read misses as the source of these slowdowns.
  OS checkpointer reads were8.04/6.48GB, despite its writeback role;
  producers' OS reads also grew alongside data-page writes (second1.29GB
  during40–50s with zero PG producer relation reads). This locates extra
  read traffic below explicit PG relation reads but does not identify
  filesystem read-modify-write, metadata, paging, or another mechanism.
  WAL initialization was absent in the initial20–30s slowdown; later
  contributed (second30–40s1.53GB initialized,2.0s cumulative fsync).
  It is not the sole initiating cause of these runs' degradation.
- New competing workload identified:160111 had autovacuum VACUUM ANALYZE
  vulkan.message_log_4_0 from about17.2s through the end;155827 had no
  message-table autovacuum during production (first observed60.6s, after
  its60.02s final). Slower run OS counters attributed about1.2GB writes
  to the autovacuum worker during30–60s and0.7GB reads to I/O workers.
  This is correlation, not proof autovacuum caused the extra foreground
  writes. Next causal intervention: keep autovacuum enabled and compare
  controlled maintenance timing/pacing with matched producer workload,
  measuring foreground evictions/write time and maintenance progress.
  Existing bgwriter sweeps did not establish a fix; don't repeat a
  leaderboard without showing which foreground cost moved.
- Timing correction: PG cumulative counters are published after work has
  progressed; checkpointer deltas cannot locate all checkpoint writes in
  their publication window. In155827 OS checkpointer reads begin10–20s,
  while PG checkpoint writes first appear30–40s with15.6s accumulated
  write time. Updated rca_analyze.py notes accordingly; earlier window
  checkpoint-byte numbers must be read as publication deltas, not proof
  of precisely timed I/O bursts. PostgreSQL describes the reporting lag:
  https://www.postgresql.org/docs/18/monitoring-stats.html#MONITORING-STATS-VIEWS
  New per-process observations provide timing evidence; they still omit
  inaccessible/exited processes. Observer snapshots cost0.06–0.12s in
  first run and no instrumentation overhead A/B has been established.
- Controlled maintenance comparison (2026-09-08): MAINTENANCE_TIMING
  after/during; matched45s producer runs,8 callers,batch1000,pool32,
  maximum12m messages. Both message leaf and idempotency table retain
  autovacuum enabled but use20m insert/analyze thresholds,scale0, so
  automatic start does not confound the interval. These are diagnostic
  thresholds, not adopted production tuning. Explicit VACUUM ANALYZE on
  message leaf starts at15s or just after production, with2ms cost delay,
  cost limit200 (current autovacuum pacing). Deferred vacuum must finish
  before verification/drop; its work is saved in maintenance.log. During
  run vacuum snapshots an earlier table size, so work volumes differ;
  record pages scanned and residual inserts rather than claiming equal
  total maintenance. Capture pg_stat_progress_vacuum and table counters.
  Initial smokes160640/160656 failed instrumentation setup (partitioned
  parent disallows reloptions; empty progress needs JSON[]); corrected,
  databases removed. Smoke160709 passed10k rows and completed vacuum.
  Measured comparisons follow only the successful smoke; no library edits.
- Deferred-maintenance arm160730 produced8,180,000/45.033s (181.6k/s).
  Ten-second windows226/211/149/134k, final5s196k. No vacuum progress
  during production. Producer data-page writes nevertheless reached
 1.08GB/13.68 cumulative seconds in20–30s and0.68GB/10.97s in30–40s;
  concurrent vacuum is not necessary for this slowdown. Deferred vacuum
  had scanned357,494/1,168,574 pages after96s, so postponement hides
  significant maintenance debt. This is not an accepted steady-state
  configuration or a zero-backlog result. Await completion and during arm.
- Maintenance harness limit:160730 production ended normally but its
  deferred VACUUM exceeded the original300s runner wait. Marked maintenance
  timing interrupted; it is not a valid uninterrupted completion duration.
  Increased future scratch wait to900s. Confirmed original maintenance PID
  absent, then resumed VACUUM ANALYZE at the same2ms/200 pacing, followed
  by row/batch verification and database cleanup before the during arm.
  Producer interval remains usable: timeout was strictly after production.
- During-maintenance arm161518 produced7,726,000/45.038s (171.5k/s),
  errors0. Manual vacuum began15.3s, heap snapshot487,995 pages; scanned
 96,288 by42.8s. Rates226/205/155/116k per10s, final5s141k. Compared
  deferred arm181.6k/s, sampled producer relation writes grew1.76→2.65GB
  and24.65→40.66 cumulative seconds. Initial10s rates matched226k/s;
  later difference is consistent with maintenance interference but one
  sequential pair cannot establish its effect size amid observed variance.
  Plan pacing intervention: same15s start/cost limit200 with delay0.2ms
  versus2ms. No durability changes, no autovacuum disable. Record whether
  quicker scan completion reduces overlap or merely adds peak contention.
- Maintenance pacing results161851:0.2ms delay produced6,867,000/
 45.081s (152.3k/s), errors0; first10s225k/s matches both controls,
  then183/128/109k, final5s83k. VACUUM scanned484,044 pages in16.71s
  versus487,995 in130.89s at2ms. Fast vacuum dirtied404,464 buffers,
  generated3.400GB WAL/442,340 full-page images and14,168 buffers-full
  events;2ms vacuum dirtied488,014 buffers, generated3.753GB WAL/
 488,015 full-page images with0 buffers-full events. Fast scan froze
 83.56% of pages versus99.84% for slow scan; timing changes completed
  work too, so this is not an identical-work throughput ratio. Sampled
  producer data-page write times: deferred24.65s,2ms40.66s,0.2ms44.37s.
  Producer DataFileWrite wait samples259/425/505 respectively. Short
  scan completion does not imply less producer interference: generated
  WAL and page writeback remain. Single sequence, no counterbalanced
  repeats; observed differences aren't confidence intervals or maxima.
- Maintenance conclusion: vacuum is an additional source of dirty pages
  and full-page WAL; controlled pacing demonstrates it can create a WAL
  buffer-pressure burst. It is not necessary for the base slowdown,
  which persisted with maintenance postponed. Next fence: isolate the
  cost of maintenance-generated full-page WAL from relation writeback
  while preserving durability, then repeat matched treatments. Do not
  adopt deferred maintenance or call these sustainable results. During
  vacuum covers its earlier heap snapshot, not all later inserts; the
  idempotency table also retains deferred maintenance. A zero reported
  n_ins_since_vacuum is not proof all late pages were scanned.
  Deferred arm cleanup confirmed first vacuum had completed (resumed
  scan visited1 page, vacuum_count2), then ANALYZE completed; retain the
  client-timeout annotation. All3 production counts/batches verified,
  scratch databases dropped, no consumers. Raw maintenance logs/configs,
  progress, backend/host counters, analysis files and runs.jsonl saved.
  Global PostgreSQL settings were never changed; per-table diagnostic
  thresholds disappeared with scratch databases. No library code edits.
- Maintenance WAL isolation: session-local MAINTENANCE_COMPRESSION
  off→lz4→off, same15s manual-vacuum start,0.2ms delay,cost limit200,
 45s production,8 callers,batch1000,pool32. Producer/global compression
  remainoff; fsync/full_page_writes/synchronous_commit remainon. This
  changes full-page-image encoding for maintenance while retaining page
  writeback work, though downstream checkpoint timing can also change.
  SHOW wal_compression in maintenance.log verifies the active session
  value; global settings and maintenance-config.json are retained. LZ4
  smoke162419 passed10k rows, verified effective setting and cleanup.
  PostgreSQL documents compression preserves recovery protection at CPU
  cost: https://www.postgresql.org/docs/18/runtime-config-wal.html#GUC-WAL-COMPRESSION
  Current1000-byte synthetic payload has repeated padding; any compression
  benefit must be qualified by payload compressibility, not generalized.
- Maintenance-only compression first results: off162439 produced
 7,026,000/45.032s (156.0k/s); LZ4162534 produced6,514,000/45.056s
  (144.6k/s), both errors0. Vacuum WAL3.521GB→194.9MB, full-page
  images457,991→442,387, buffers-full7554→0, scan20.63→6.07s.
  LZ4 reduced the targeted WAL cost but did not remove the producer
  slowdown: its final15s ran75.5/48.3k/s, WAL writes about1.1/1.7ms,
  producer relation writes accumulated46.15s in30–40s across8 backends.
  Off corresponding interval19.88s. Dirty buffers and freezing also
  changed with scan timing (LZ4105,152 dirtied,22.19% pages frozen), so
  this is not perfectly identical page work. Await final off control;
  do not infer LZ4 itself causes the lower rate from one middle run.
- Compression bracket complete: finaloff162651 produced6,808,000/
 45.024s (151.2k/s), errors0; vacuum3.390GB WAL/442,578 full-page
  images/15,350 buffers-full,15.29s scan. Off→LZ4→off rates156.0→
 144.6→151.2k/s; vacuum WAL shrank about94% with LZ4 yet slowdown
  persisted. Sampled producer relation-write time32.32→74.54→32.63s.
  This fences out maintenance WAL volume/buffer pressure as a sufficient
  explanation for the base collapse. It does NOT establish that global
  WAL compression is harmful or unhelpful, nor that these runs isolate
  its causal throughput effect: only maintenance used LZ4, scan timing
  and amount of freezing changed, and host/storage variance remains.
  Strongest unresolved path is foreground data-page writeback and the
  OS-attributed read traffic observed during writes. Next bounded test:
  compare matched8KiB/16KiB file rewrites to see whether smaller page
  updates cause extra host reads; do not assume an OS/filesystem mechanism
  until measured. Preserve native database semantics and durability.
  All3 full runs plus smoke verified rows/batches, no consumers/errors;
  scratch databases removed. Global compression stayedoff throughout,
  per-session setting confirmed via SHOW. Python syntax/diff checks pass;
  raw evidence and index retained. No new production setting adopted.
- Page-write-size isolation: storage_isolate.py --data-block-kib8/16,
  mixed250MiB/s1MiB O_DSYNC writer plus300MiB/s buffered data writer,
  fsync every second, fresh8GiB data file then sequential reuse. Four45s
  runs8→16→16→8; compare equal time windows1–24s growth and30–44s
  reuse. Smoke16KiB passed3s at target300MiB/s with effective block size
 16384 recorded. No explicit file reads in writer; OS counters still
  include filesystem/kernel work. Changing write size also changes syscall
  count, so throughput alone cannot identify a read-modify-write mechanism.
  Auto-review rejected initial copy-based orchestration as potentially
  retaining >100GB. Verified binary unlink and retained storage11.3GB,
  replaced copies with direct evidence-directory runs, asserts zero binary
  files before/after each and <100MB retained evidence per run. Approved
  retry uses one24GiB capacity at a time plus existing retained storage.
- Matched write-size results8→16→16→8 (163203/163249/163335/163421):
  all data writers achieved300MiB/s during1–24s file growth. In30–44s
  reuse,8KiB writes fell to156.2/141.6MiB/s with2.369/1.995GB OS
  reads, roughly equal to their2.376/2.000GB OS writes.16KiB writes
  stayed300.0/300.0MiB/s with86,016 bytes read in each window and
 4.267/4.283GB written. Reversal reproduced the difference. Durable
  writer target remains250MiB/s; no PostgreSQL workload in these controls.
  This is strong evidence that write granularity affects hidden read
  traffic in this local storage path, but fewer syscalls is a second
  changed variable. Follow-up holds16KiB size and shifts offsets by8KiB
  to test page boundaries at the same syscall rate/byte target. Fresh
  file,45s, no copies, binary capacity24GiB+8KiB, cleanup asserted.
- Alignment control163538 completed:16KiB writes shifted8KiB retained
 300MiB/s growth, but reuse fell152.1MiB/s with2.130GB OS reads and
 2.126GB writes. Aligned16KiB controls held300MiB/s with86KB reads.
  Same write size, target byte rate and fsync cadence: boundary alignment
  reproduces hidden reads and slowdown without doubling syscall count.
  This establishes an alignment-sensitive rewrite cost in the tested
  local storage path. It does not yet establish its exact OS/filesystem
  implementation or what fraction of PostgreSQL's slowdown it explains.
  Next: inspect/observe PostgreSQL's actual write sizes and offsets,
  including whether adjacent8KiB pages are combined or written separately,
  then test a supported way to improve that path. Do not assume changing
  PostgreSQL page size is a configuration toggle or propose disabling
  durability. Synthetic target rates are not device maximum benchmarks.
  All5 controls completed, binary cleanup assertions passed; no database
  settings/code changed. Source snapshots/hashes, configs, host counters,
  per-second latencies, phase analysis and runs.jsonl retained. Minimum
  host free in alignment control139.4GB; all run files fit authorized cap.
- Native write-path audit (2026-09-08): installed PG18.6 headers set
  BLCKSZ8192 and HAVE_DECL_PWRITEV1; binary imports both pwrite/pwritev.
  No missing macOS vectored-I/O support. REL_18_6 FlushBuffer calls
  smgrwrite, whose installed inline wrapper passes nblocks=1 to smgrwritev.
  Native disassembly independently shows mov w4,#1 before _smgrwritev
  and8192-byte I/O accounting. mdwritev computes offset=8192*blocknum
  within1GiB segment; FileWriteV calls pg_pwritev, whose iovcnt1 branch
  calls pwrite. Both producer eviction and checkpointer/bgwriter
  SyncOneBuffer use this single-page FlushBuffer path. Adjacent dirty
  pages are not combined there; io_combine_limit is not consulted.
  Example offsets: page42→344064 (16KiB aligned), page43→352256
  (8KiB into next16KiB boundary); illustrative offsets, not captured IDs.
  Source: https://github.com/postgres/postgres/blob/REL_18_6/src/backend/storage/buffer/bufmgr.c#L4031
  https://github.com/postgres/postgres/blob/REL_18_6/src/backend/storage/smgr/md.c#L997
- Corroboration:15 backend/run aggregates across155827/160111/162439/
 162534/162651 all report exactly8192 relation bytes/write operation.
  Source+installed binary+PG counters establish submitted write path;
  not a syscall histogram. Read-only sudo -n fs_usage attempt could not
  run because local sudo requires a password; no credentials requested
  or privilege/settings changes made. Artifact write_path_audit contains
  binary hash, build config, installed headers, FlushBuffer/FileWriteV
  disassembly and measured-write-sizes.json. No new load tests this audit.
- Configuration boundary: no ordinary parameter coalesces this specific
  single-page flush path in18.6. debug_io_direct uses F_NOCACHE on macOS
  but PostgreSQL labels it developer-testing-only, not a production fix:
  https://www.postgresql.org/docs/18/runtime-config-developer.html#GUC-DEBUG-IO-DIRECT
  Supported --with-blocksize=16 requires a separate build/new cluster and
  is not pg_upgrade-compatible with8KiB storage. Clean next experiment:
  matched8KiB/16KiB builds of18.6, same compile options and byte budgets,
  isolated fresh clusters, unchanged WAL block size and durability. Test
  actual Vulkan writes and OS reads; larger pages also change row packing,
  cache page count and full-page WAL, so avoid attributing all rate change
  solely to alignment. Preserve current native cluster as the baseline.
  https://www.postgresql.org/docs/18/install-make.html#CONFIGURE-OPTION-WITH-BLOCKSIZE
- Matched PG page-size builds authorized (2026-09-08): official18.6
  tarball SHA256555610c24d53e4316da5b7d3fc25c279d96856d5e0e23ee308c328c5fa881d9f,
  extracted under/private/tmp/vulkan-page-builds. Sameclang/O2/ICU/LZ4/
  Zstd/OpenSSL options, differing only install/build paths and8/16KiB
  data block size; WAL block size unchanged. No source patches. First
  configure found pkg-config absent; use explicit installed ICU/include/
  library paths instead, no system dependency installation. Build logs and
  command/env manifests retained. Need installed pg_stat_statements and
  pg_buffercache for each ABI. run.py accepts dedicated root override;
  existing session-path pointer and original cluster remain unchanged.
  New root storage guard75,000,000KiB includes both clusters/builds; add
  existing~11.3GB retained leaves headroom under100GB authorization.
- Page-size test setup: page_compare.py initializes isolated data8/data16
  clusters (UTF8,C locale,checksums on), installs matching contrib modules,
  asserts block_size8192/16384,shared_buffers6442450944 bytes,
  wal_buffers16777216 bytes and fsync/full_page_writes/synchronous_commit/
  checksums on before every run. Background writer100/50 pages per200ms
  keeps byte allowance equal; max/min WAL8/2GB,target0.9,work_mem32MB,
  maintenance64MB,vacuum ring2MB,combine limit128KB. Other settings match
  between builds; automatic maintenance remains enabled with ordinary
  defaults (its page-based costs can still differ). No controlled manual
  vacuum. Smoke10k each, then45s8→16→16→8,8 callers,batch1000,pool32.
  Only one native cluster on port55439 at once. Original cluster is stopped
  temporarily and restarted in finally; baseline files/settings preserved.
  Every scratch DB verified/dropped before the next; no cumulative dataset
  retention. Builds/root covered by75mKiB guard, original clusters separate.
- First matched page-size pair:8KiB165031 produced8,783,000/45.024s
  (195.1k/s),16KiB1652338,779,000/45.027s (195.0k/s), errors0.
  OS checkpointer reads5.355GB→28,672 bytes; sampled producer relation
  write time9.569→0.0925 cumulative seconds. Actual database confirms the
  synthetic alignment penalty is removed, but first-pair throughput did
  not rise. PG16KiB used about4.46 CPU cores, app0.90, mean BufferContent
  waits1.05 sessions, negligible pool waiting; no autovacuum samples in
  either production window. Equal page-size comparison uses new binaries,
  not Homebrew-vs-custom compiler differences.
-16KiB repeat165447 produced7,323,000/45.701s (160.2k/s), errors0;
  checkpointer reads57,344 bytes, producer page-write time0.0691s.
  Nevertheless final windows fell109k then59k/s, WAL5.09/5.77ms per
  write, WALWrite waiters3.19/5.26. This reproduces durable-WAL stalls
  after data-page alignment cost has been removed. Do not conclude CPU
  is the sole next bottleneck from the faster16KiB run. First-use versus
  repeated cluster/WAL state also differs; record WAL initialization and
  reuse when interpreting the final8KiB control. No end-to-end maximum.
- Matched8→16→16→8 completed: final8KiB165658 produced6,735,000/
 45.016s (149.6k/s), errors0; checkpointer reads3.692GB, sampled
  producer data-write time8.536s. All4 measured runs verified rows:
  exact counts8,783,000+8,779,000+7,323,000+6,735,000=31,620,000;
  plus two10k smokes. No consumers. Rates8KiB195.1/149.6k versus
 16KiB195.0/160.2k. Data-write time8KiB9.57/8.54s versus16KiB0.093/
 0.069s; checkpointer reads8KiB5.36/3.69GB versus16KiB29/57KB.
  This validates alignment-related read/write overhead in real PostgreSQL,
  but does not establish a higher overall ceiling or sustainable maximum.
  The remaining WAL-stall regime appears in both page sizes; final8KiB
  WAL writes reached6.05–6.66ms in slow windows. Build-page-size change
  also affects row packing, cache pages, full-page WAL and page-based
  maintenance costs; comparison is not purely an alignment switch.
- Next WAL fence: use16KiB build to suppress established data-page penalty
  and compare normal WAL recycling against fresh segment allocation with
  durability preserved. First runs initialized10.25/8.61GB WAL and were
  fast; repeats initialized0/1.29GB yet stalled. This makes initialization
  itself an unlikely sole cause and file reuse a hypothesis, not proof;
  host state and checkpoint timing still confound chronology. Avoid
  drawing a permanent CPU-bottleneck conclusion from only faster runs.
  Original Homebrew8KiB cluster restarted and verified6GB/durability ON,
  global compressionoff; comparison clusters stopped (no postmaster.pid),
  their scratch databases removed. Retain binaries and empty clusters for
  follow-up, total dedicated storage about29GB including builds/evidence,
  host free142GB. Build manifests/hashes/configs and comparison.json under
  evidence/native18/page_builds; per-run analysis and runs.jsonl retained.
  Python syntax and scoped diff checks passed; no library code changed.
- WAL recycling investigation completed: matched16KiB build, recycling
  ON -> OFF -> ON; each arm first produces8m messages (90s cap), then
  measures45s with a fresh database. Restart between runs, retain WAL
  files, keep wal_init_zero and all durability settings ON. Assert the
  effective setting on every start; retain phase.json and the plan under
  evidence/native18/page_builds/wal_recycling_plan.json. Compare WAL
  initialization counters, write latency, waits, and time-window rates;
  fixed-count conditioning does not guarantee identical filesystem state.
- Recycling results (2026-09-08): measured ON170645 / OFF171039 /
  ON171433 =200,728 /141,249 /125,214 messages/s over45.04/45.28/45.18s.
  Conditioning170447/170906/171157 each reached8m rows in38.55/49.92/
  61.84s. All45,094,000 rows across six runs verified, batches1000,
  zero errors/duplicates, no consumers or exception instances; DBs dropped.
  The setting took: measurement WAL-init bytes84MB/9.60GB/17MB,
  initialization fsync time0.048/5.163/0.034s. OFF really allocated fresh
  segments, and final ON returned to reuse. OFF still dropped to94.5k/s
  with6.03ms WAL writes and3.52 mean WALWrite lock waiters in20–30s;
  final ON reached80.3k/s with9.74ms WAL writes in30–40s and52.3k/s
  in its final5s. Recycling is not necessary for the severe stall regime,
  and disabling it is not a demonstrated fix. No recovery on return to
  ON means chronology/storage state confounds the independent setting
  effect; do not claim OFF itself costs30% from these averages.
  Measurement PG CPU4.57/3.14/2.78 cores, app0.89/0.70/0.61 cores;
  post-warmup pool wait rounded0s, median8 acquired connections throughout.
  CPU exhaustion and pool starvation do not explain this slowdown.
  Aligned-data benefit persists: checkpointer OS reads20/37/8KB;
  producer data-page write time0.059/0.287/1.021 cumulative seconds.
  Autovacuum overlapped parts of runs and is not a controlled constant.
  Backend/global counters can publish late; these are not syscall traces.
  Next fence: investigate WAL write alignment/size versus changing host
  storage state. Both page builds retain8KiB WAL blocks; prior synthetic
  WAL controls used1MiB aligned writes, which do not cover every PostgreSQL
  WAL write boundary. Compare controlled write boundaries with equal
  bytes/durability and repeated ordering before attributing stalls to SSD
  hardware, APFS, or PostgreSQL. Keep maintenance timing visible.
  Evidence: page_builds/wal_recycling_comparison.json, per-run analyses,
  runs.jsonl. Baseline restored and verified durability/recycling ON,6GB
  shared buffers, no scratch DBs; both comparison clusters stopped.
  Retained storage29.1GB, host free138.6GB; restored-state evidence in
  page_builds/wal_recycling_restored.json. Python syntax checks passed;
  no library changes or sustained/end-to-end maximum claimed.
- Continued root-cause investigation: storage-only WAL offset0/8/8/0KiB
  controls, prefilled8GiB WAL ring,1MiB O_DSYNC writes250MiB/s, aligned
  16KiB data writes300MiB/s to8GiB ring,45s each. Stop/restore baseline;
  no DB workloads overlap. Retain source/config/counters in wal_alignment_*
  evidence directories, delete both temporary files between arms.
  PostgreSQL XLogWrite uses nbytes=npages*XLOG_BLCKSZ and offsets on WAL
  page boundaries (xlog.c2394/2422/2434); data16KiB build still uses8KiB
  WAL. This synthetic test isolates fixed offset, not its full distribution.
  Next test data fsync cadence: previous synthetic controls sync every1s,
  which can hide dirty-write accumulation before checkpoint synchronization.
- WAL offset controls completed132203/132251/132339/132428: first aligned
  and two shifted runs held250MiB/s WAL with maxima7.16/10.64/13.00ms.
  Shifted WAL processes incurred181/182MB reads versus3.1MB first aligned,
  but no >20ms WAL intervals. Final aligned repeat instead reproduced
  severe stalls:205.5MiB/s WAL,520ms max,26 intervals >20ms; data297.3MiB/s,
  max518ms writes and1.795s fsync. Thus PostgreSQL/SQL/locks are not
  necessary for this stall pattern, and shifting WAL offsets is not
  necessary either. No thermal/performance warning reported by pmset;
  measured swapouts unchanged during final run. No large accessible
  non-benchmark process I/O found; inaccessible processes remain a limit.
  Follow-up storage_cache_*:60s each, data sync30s cached /1s F_NOCACHE /
  1s cached, otherwise identical. F_NOCACHE48 verified in installed SDK.
  PostgreSQL18 also exposes this via debug_io_direct (fd.c1144); this is
  a diagnostic-only setting, not a proposed production configuration.
- Storage-cache controls132723/132827/132931 completed: WAL250.0/250.0/
  221.7MiB/s, max8.55/7.71/464ms, >20ms intervals0/0/52; data298.7/
  300.0/277.5MiB/s. Delayed fsync alone did not reproduce the stall;
  cache bypass then return to cached behavior is suggestive, not yet a
  causal estimate. Native follow-up runs45s with debug_io_direct empty /
  wal /data,wal /empty, matched16KiB build, WAL recycling/durability ON.
  All include the same10MiB/s independent O_DSYNC probe, and assert the
  actual debug_io_direct value. Plan in page_builds/direct_io_plan.json.
- Native direct-I/O diagnostics173158/173340/173603/173806 completed:
  default /wal /data,wal /default =207,992/173,601/150,069/134,687msg/s
  over45s,30,046,000 verified rows, errors/duplicates0, no consumers.
  Independent10MiB/s probe maxima41/600/411/8345ms (whole probe interval;
  inspect timestamps for production overlap). WAL-only bypass retained
  late5.07ms WAL writes; data+WAL bypass shifted toward relation extension
  (mean1.82 extension-lock waiters), while external probe still stalled.
  Final default30–40s20.4k/s,6.15 mean WALWrite waiters,11.38ms WAL writes.
  Cache bypass is not a demonstrated fix; neither PostgreSQL nor cached
  WAL alone is necessary for the external stalls. Live sample173158
  captured XLogWrite -> pwrite; no kernel stack was captured. Source and
  per-run analyses plus direct_io_comparison.json retained. Baseline restored.
  Next storage_switch_*90s alternates data F_NOCACHE every15s in one
  process/file to reduce between-run state confounding. Added driver
  IOBlockStorageDriver and APFS data-volume statistics every~1s; SDK
  IOBlockStorageDriver.h documents Total Time as cumulative nanoseconds.
- Within-run cache switches134302:90s, WAL250.0MiB/s and data299.1MiB/s,
  WAL max27.7ms with two >20ms intervals, no data >20ms intervals. Neither
  cached nor nocache phases reproduced severe stalls: earlier separate-run
  cache reversal does not establish causation. Retain device/APFS counters.
- Driver-instrumented native174519:15,710,000 verified messages in91.676s
  (171,364/s), zero errors/duplicates, no consumers, baseline restored.
  Producer windows227.6k initially,183.0k at60–70s,107.1k at70–80s,
  64.1k at80–90s. WAL writes0.68ms initially to17.28ms in80–90s;
  driver write request averages0.30–0.52ms through70s,13.87ms at70–80s,
  26.95ms at80–90s. Driver write throughput fell~834 to491 to294MB/s.
  This observes the slowdown in the block-storage driver below APFS,
  not merely in PostgreSQL locks or user-space syscall timing. It does
  not distinguish controller queueing, firmware/NAND behavior, or all
  separate flush commands. No claim of SSD garbage collection proven.
  APFS data-volume metadata reads rose31/61MB in final windows; user reads
  348/877MB and block-device reads375/1060MB. All volume traffic is counted.
  Driver instrumentation and device-analysis.json retain exact deltas.
  Follow with storage_rate_*90s at data600/300/600MiB/s plus WAL250MiB/s,
  aligned writes,1s data fsync, same8GiB rings, PG stopped/restored. This
  tests whether aggregate write load alone reproduces driver latency.
- Storage-rate results135030/135204/135339: high850 /low550 /high850MiB/s
  requested, all90s. First high delivered WAL250+data600MiB/s with driver
  write averages0.56–1.35ms; low delivered250+300 with two WAL >20ms
  intervals (max305ms). High repeat delivered159.2+334.3MiB/s, WAL max544ms,
  79 >20ms intervals, data max522ms. Driver write averages3.5–23.8ms,
  actual writes427–757MB/s; reads only1–5MB/s. This reproduces driver-level
  write delay without PostgreSQL, consumers, range locks, SQL, or the
  benchmark's du scan. A fixed aggregate bandwidth ceiling alone is not
  an adequate explanation: the first identical high run sustained its rate.
  Per-run device-analysis.json and page_builds/storage_rate_comparison.json
  retain timing/byte/operation deltas and syscall-latency summaries.
  Observed active synthetic workers were not Darwin-background processes;
  execution context disk policies default0, QoS33. Explicit IMPORTANT1
  priority control140238 records both worker policies0, QoS33, background0.
  Separate set/get check proves this OS normalizes IMPORTANT1 to DEFAULT0;
  STANDARD5 reports5, restoring1 reports0. All calls succeeded. Thus this
  is an already-unrestricted control, not evidence of a priority upgrade.
  Initial orchestration used wrong taskpolicy path and exited before any
  workload/files, restored baseline; corrected to /usr/sbin/taskpolicy.
  No DB errors. Automatic-review timeouts resolved on authorized retries.
- Priority control140238 delivered250+599.7MiB/s for90s with WAL max14.4ms
  and no >20ms intervals. Effective policy remained the same unrestricted
  policy as defaults; intervening idle time/state is a confound, not a
  priority improvement. Trial140211 never launched (wrong executable path)
  and created no storage files; no measured result assigned to that name.
  Final storage_recovery_* control holds files/config constant for270s:
  write120s, idle30s, write120s, same WAL250/data600MiB/s nominal targets.
  Records actual pause boundaries. Existing lifetime pacing can catch up
  a prior deficit after delays; assess achieved rates and driver timings,
  not nominal target alone. No PG workload overlaps; files remain bounded
  at16.1GiB regardless of cumulative submitted bytes.
- Recovery control140659 completed with unchanged files/config: driver
  write averages0.59/0.66ms in first two30s windows,4.07ms at60–90s,
  11.20ms at90–120s. Both workers paused at120s and resumed at150s.
  160–180s averaged1.56ms with WAL261/data605MiB/s; subsequent30s windows
  rose7.45/10.27/15.24ms, ending WAL114/data346MiB/s. Idle partly restored
  performance, then sustained writes recreated the slowdown. First resumed
  10s caught up pacing debt at595/1192MiB/s; do not treat it as a controlled
  850MiB/s interval. Driver timing and actual achieved rates are retained.
  The pause can drain queues and permit filesystem/device background work;
  it cannot identify NAND cache, garbage collection, or thermal internals.
- Root-cause boundary established for the severe throughput collapse:
  prolonged writes cause large completion delays in the Mac's block-storage
  driver/device path, also reproduced without PostgreSQL. PostgreSQL holds
  WALWriteLock through XLogWrite/pwrite; a delayed durable write serializes
  other producers behind it. This explains WALWrite queues and falling CPU
  despite spare connections. Separately,8KiB data-page alignment causes
  read/write amplification already removed by the16KiB diagnostic build.
  Fixed SSD bandwidth, specific firmware/NAND mechanism, and maximum
  sustainable end-to-end throughput remain unproven. Cache/recycling/priority
  changes are not established cures. du scans may add observer overhead,
  but are absent from the reproducing storage-only control and hence not
  necessary for the collapse. Do not infer application's absolute maximum
  from a host whose storage rate changes during nominally identical runs.
  Summary and exact evidence mapping: page_builds/root_cause_findings.json.
  This continuation verified45,756,000 native producer messages with zero
  errors/duplicates, no consumers; all scratch DBs dropped. Baseline restored
  with6GB buffers, default caching/recycling and durability enabled; test
  clusters stopped, all synthetic .bin files removed. Retained storage29.2GB,
  host free132.7GB, restoration evidence in root_cause_restored.json.
  Python syntax/scoped whitespace checks passed. No library changes/commits;
  user's concurrent repo reorganization preserved; notes followed docs/ to
  .docs/ when moved, per updated AGENTS.md.
- [x] Deeper storage investigation, 2026-09-08: built official smartmontools
  7.5 in /private/tmp/vulkan-smartmontools, no installation or daemon.
  Health-only `smartctl -A -j /dev/disk0` works without sudo; full -a exits4
  because Apple rejects its error-log query, so monitoring uses -A only.
  Temperature responds to load (29 to36C initially); no critical warning or
  media error. Ongoing control storage_smart_151534 adds5s health samples to
  the same270s write/pause/write experiment. Controller properties identify
  TLC NAND,16KiB NAND pages, and64 driver command objects; these properties
  alone do not establish a cache limit or a measured queue depth.
  Audited Apple IOBlockStorageDriver upstream source: Total Time (Write)
  starts before deblocking/breakup and ends at request completion. It includes
  lower-driver queueing and transfer, not solely flash service time; separate
  doSynchronize calls are outside this write counter. Source/version limits
  and hashes archived in page_builds/telemetry-provenance.json. The io_rate
  callback is registered outside the published implementation, so its name
  alone cannot rule software throttling in or out. Next control removes the
  second writer and small writes, using configurable WAL byte rate and a
  no-catchup pacer; no library changes.
- [x] SMART-instrumented mixed control storage_smart_151534 completed:
  first60s driver writes891-915MB/s at0.67ms mean completion;90-120s
  fell to548MB/s at17.22ms, temperature35-37C. After30s pause, writes
  returned to1338MB/s at1.12ms while warmer(37-39C); early recovery
  rate includes old pacer's catchup debt. Last30s700MB/s at8.16ms,
  temperature37-39C. WAL/data maximum calls528/574ms, zero SMART
  warnings/media errors. A simple composite-temperature threshold fails
  to explain these periods; firmware-internal temperatures/counters remain
  unavailable. Matching raw samples,30s windows, and limits in analysis.json
  and runs.jsonl. Next storage_sequential_152034 uses one1MiB O_DSYNC
  writer,850MiB/s target, same8GiB file throughout, no catchup pacing,
  120s write/30s pause/120s write. This tests whether high byte volume
  suffices without a second writer or small-write syscall pressure.
- [x] Single-writer control storage_sequential_152034: one1MiB O_DSYNC
  caller targeting850MiB/s,8GiB reused file,120s write/30s idle/120s
  write, no-catchup pacing. Active mean843.4MiB/s; ordinary30s driver
  windows860-896MB/s and0.22-0.28ms. Three sampled intervals exceeded
  20ms (maximum328ms), but no prolonged throughput collapse. Temperature
  mostly36-38C. This excludes a universal sustained850MiB/s byte ceiling
  for this access pattern, not an SSD bottleneck for other access patterns.
  The smaller8GiB live file versus16GiB across mixed files is a confound;
  storage_sequential16_152538 changes only WAL file capacity to16GiB with
  the same270s schedule. Do not attribute the difference to concurrency
  or SLC cache until this size control and matched mixed trials complete.
- [x] Larger single-file control storage_sequential16_152538 completed:
  active mean812.7MiB/s. Most30s windows866-892MB/s at0.23-0.26ms
  driver time, but final30s628MB/s at1.03ms, with25 sampled WAL
  intervals above20ms(max343ms). Thus sequential writes also eventually
  slow; the preliminary live observation that16GiB stayed fast does NOT
  exclude size/history effects. The8GiB control had only3 long intervals.
  Planned matched buffering comparisons recorded in buffering-plan.json:
  16KiB cached,1MiB cached,1MiB no-cache,1MiB cached repeat;150s each,
  same250+600MiB/s targets,8+8GiB files, page-aligned anonymous buffers,
  no catchup pacing. Source audit of XNU vfs_cluster.c confirms IO_SYNC
  bypasses delayed clustering and waits; no-cache direct selection depends
  on buffer alignment/request size. Published source is not a runtime trace.
  Prior policy0/QoS33/DarwinBG0 checks report requested policy, not full
  effective policy (Apple task_policy.c states this explicitly). Cross-process
  task_for_pid read attempt failed5 on both writers; TASK_POLICY_STATE's
  source requires privilege. New workers query their own suppression/category
  and effective-state return codes so missing permission is explicit.
- [x] Matched buffering results recorded so far: small_cached_153053
  WAL/data209.8/541.7MiB/s, maxima496/484ms; large_cached_153330
  112.0/375.6MiB/s, maxima712/604ms; large_nocache_153606
  187.0/370.4MiB/s, maxima496/496ms. Aligned1MiB no-cache data
  writes still collapse; delayed OS data caching and small syscalls are
  not necessary. Mean driver completion falls versus cached(1.9-4.8ms
  versus8.4-11.9ms), but host chronology and delivered rates limit effect
  attribution. Own-task category/suppression query succeeds with all zero
  fields at no-cache startup; effective-state query returns2(protection).
  spindump -onlyTarget attempted during cached repeat returned77:
  live sampling requires root. This is macOS privilege enforcement, not
  automatic approval rejection. No external physical disk is attached.
  Final cached repeat is running; next working-set-plan.json changes
  only overwrite range16/1GiB every60s in a fixed16GiB file, no-cache,
  one aligned1MiB O_DSYNC writer,850MiB/s cap, no pauses or catchup.
- [x] Cached repeat large_cached_repeat_153843 reproduced114.0/379.1
  MiB/s(WAL/data),870/516ms maxima: combined493.1 versus487.6MiB/s
  initial large-cached, with557.4MiB/s for no-cache data. The no-cache
  case still stalled; no universal cache-bypass cure. All four cases and
  settings in buffering-comparison.json and runs.jsonl.
- [x] Same-file active-range experiment storage_working_154235 completed:
  six60s phases16/1/16/1/16/1GiB within one continuously allocated
  16GiB file, one page-aligned1MiB O_DSYNC+F_NOCACHE caller,850MiB/s
  target. Every phase848-849MiB/s, maximum sampled write12.5ms, zero
  sampled intervals>20ms. Neither total byte volume nor16GiB active
  overwrite range forces the observed collapse in this direct sequential
  access pattern. Do not claim a measured SLC capacity or universal SSD
  post-cache bandwidth limit. Next concurrent-direct-plan.json tests two
  direct1MiB writers,8+8GiB files, rate splits250+600,425+425,
  250+600MiB/s(A/B/A),150s each. This removes the remaining WAL-cache
  difference and probes whether mixed rewrite frequencies matter.
- [x] Both-direct rate-split comparison: unequal_154915250/600 targets
  delivered249.7/598.2MiB/s, equal_155151425/425 delivered424.4/424.3,
  unequal_repeat_155427 delivered249.1/595.6. Full150s periods near
  target; long sampled WAL intervals1/0/2, maxima45/8/316ms. A few
  isolated stalls remain, but none had the prolonged collapse seen when
  the synchronous writer used the cached path. File capacity, request
  sizes and total target matched; differing chronology still warrants a
  same-file intervention before claiming a cache cure.
  Published XNU11417.140.69 vfs_cluster.c lines3062-3070 confirms
  unaligned file offsets route the entire direct-write vector through
  the copy/cache path. Running kernel adds708.3 patch suffix, so exact
  downstream binary tracing remains unavailable. PG18.6 XLogCtl buffers
  align to XLOG_BLCKSZ and WAL positions advance in8KiB units; our16KiB
  data-page build still has8KiB WAL blocks. A requested no-cache flag
  therefore does not guarantee the kernel's direct path for every request.
  Next storage_offset_direct_155859 switches only WAL displacement
  0/8192/0/8192/0 bytes every60s, same8+8GiB files, both no-cache,
  aligned1MiB buffers,250+600MiB/s targets. Extra1MiB was prewritten
  to avoid file extension at shifted wraparound. Cache-toggle trial is
  deferred in favor of this more specific alignment test.
- [x] Same-file alignment trial storage_offset_direct_155859 completed:
  WAL offset0/8192/0/8192/0 in five60s phases delivered249.45/232.81/
  249.13/180.14/240.73MiB/s. Median WAL write latency0.330/1.568/
  0.323/3.007/0.323ms; shifted phases introduced457.7/351.0MB OS
  reads versus essentially zero in aligned phases. Alignment causes
  read amplification and higher typical latency. The second shifted
  phase reached456ms and the final aligned phase still reached222ms:
  alignment alone does not explain prolonged stalls. Raw phase-analysis
  and settings are in the run directory and runs.jsonl.
- [x] Same-file cache trial storage_wal_cache_160503 completed: WAL
  F_NOCACHE on/off/on/off/on, five60s phases, aligned1MiB buffers and
  offsets, data always no-cache. WAL rates249.76/249.60/236.97/237.26/
  214.45MiB/s; data595.43/596.68/540.70/512.93/422.14MiB/s. The
  largest WAL stall870ms occurred after bypass was restored. This
  contradicts a universal instantaneous cache-bypass cure; delayed OS
  work or controller behavior remain hypotheses, not established causes.
- [ ] Capture kernel stacks during a reproduced stall to separate OS
  queueing/throttling from lower-driver/controller waits. Normal-user
  spindump requires root (exit77); sudo -n reports a password is required.
  These are macOS privilege limits, not automatic approval rejection.
  Asked whether the user can execute a narrowly scoped10s administrator
  trace in their own Terminal; no password is requested or handled here.
- [x] Restored original native PG18.6 with durability and autovacuum on;
  both custom instances stopped and all synthetic .bin files removed.
  deeper-investigation-restored.json records29.42GB retained and130.92GB
  free. No application throughput maximum is established by these
  storage-only tests under `.bench/scratchnative`.
- [x] Native WAL alignment comparison: build matched PG18.6 with16KiB
  data pages and16KiB WAL blocks, versus existing16KiB data/8KiB WAL.
  Run8/16/16/8KiB WAL order,60s each, WAL no-cache enabled, fixed
  producer and durability settings. This tests alignment cost in real
  production; it does not assume alignment explains all long stalls.
- [x] Follow WAL alignment repeats with one60s16KiB WAL comparison
  using fsync_writethrough (F_FULLFSYNC on macOS), keeping WAL no-cache
  and all durability settings on. This changes synchronous-open writes
  into explicit durable flushes; compare write and fsync latency separately.
- [x] SSD ceiling sanity check requested by user: confirmed MacBook Air
  Mac16,13/M4/24GB, internal APPLE SSD AP0512Z512GB, revision2914.80,
  TRIM enabled. Published512GB M4 Air Blackmagic results around3-3.5GB/s
  are short sequential tests, not durable database ceilings. Prior849MiB/s
  test was capped at850MiB/s and is only a lower bound. Add unpaced
  scratch mode and measure aligned1MiB synchronous no-cache writes on
  this machine with PG stopped,16GiB fixed footprint,120s maximum.
  Sync-method follow-up remains queued after this requested sanity check.
- [x] Native WAL ABBA completed: scratch_204217/204500/204719/204907,
  WAL8/16/16/8KiB, data16KiB, WAL no-cache,60s each. Rates191.9/
  165.2/131.8/129.8k/s: chronology dominates; no throughput cure proven.
  Producer-attributed OS reads159.86/0.69/0.05/78.92MB support the
  WAL alignment penalty. Total PostgreSQL reads1774/0.83/689/835MB
  also include io-worker reads and must NOT all be attributed to WAL.
  First16KiB WAL early write latency0.33ms versus8KiB0.64ms, but the
  repeated16KiB run fell228k->75k/s with WAL writes8.81ms near end;
  driver completion0.71->25.17ms and driver throughput605->337MB/s.
  Effective block sizes, byte budgets and all durability flags verified.
  Zero producer errors/duplicates and no consumers/workers; row count,
  batch and semantics checks passed; scratch databases dropped. Full
  windows/settings in page_builds/wal-block-comparison.json and runs.jsonl.
  bench moved during first run; a temporary compatibility symlink let
  that run archive into .bench, then was removed before later runs.
- [x] storage_ceiling_205218: unpaced single aligned1MiB O_DSYNC+
  F_NOCACHE writer, PG stopped,16GiB prefilled overwrite file,120s.
  Short intervals reached2.9GB/s but mean944MB/s (900MiB/s); driver
  30s windows1221/1108/728/715MB/s, max sampled write533ms,
  75 sampled intervals exceeded20ms. Composite temperature45-48C
  early versus43-44C late, with no health warning. This is a measured
  workload-specific result, not a universal SSD physical ceiling.
  Earlier850MiB/s capped trials establish a lower bound only. Published
  comparison:512GB M4 Air Blackmagic3456.2MB/s write, source/provenance
  in page_builds/ssd-reference.json and ssd-hardware.txt. Short sequential
  benchmark throughput cannot be equated to mixed durable database I/O.
  Synthetic file removed; run/config/script/health recorded in runs.jsonl.
- [x] scratch_205451 fsync_writethrough comparison completed105.0k/s
  over60s: first20s132-136k/s, last20s53-67k/s. WAL write means
  stayed0.38-0.59ms but explicit flush means3.7->8.1-9.4ms;
  relation extension cost also rose. Stronger explicit flush did not
  cure collapse. All row/batch/semantics checks passed, zero errors or
  duplicates, no consumer/worker instances, scratch DB removed.
  wal-sync-comparison.json contains detailed windows; runs.jsonl has
  configuration, source/binary hashes and validation evidence.
- [x] Restored Homebrew PG18.6 baseline8KiB data/8KiB WAL, original
  open_datasync, cache bypass off, all durability/autovacuum settings on.
  All three custom clusters stopped. wal-and-ceiling-restored.json
  records38.22GB retained,121.85GB free, no synthetic .bin files.
  Kernel tracing remains the concrete missing discriminator: user-side
  administrator execution is required by macOS; no password available
  to sudo -n. Do not label NAND cache exhaustion/GC as proven or claim
  the current database throughput is an unavoidable physical maximum.
- [x] Prepared .bench/scratchnative/capture_kernel.py for user Terminal
  execution. sudo -n remains blocked by a required password. The script
  authenticates with sudo in Terminal, runs the existing storage-only
  writers as the normal user, and captures a10s two-process spindump
  after a sampled write exceeds100ms. Three-minute workload,16GiB
  footprint, storage/free-space guards, cleanup and PG restoration.
  Only tracing uses root. Syntax checked; live privileged capture remains
  pending user execution. No kernel-stack finding claimed yet.
- [x] User captured storage_kernel_20260908_210430 successfully.
  Trace17:05:18.789-17:05:28.779: WAL writer972/999 samples in
  buf_biowait below APFS/cluster_write (97.3%),0.063s CPU. Data
  writer470/999 samples in vnode_waitforwrites from pwrite plus47
  from fsync;378 samples are intentional pacing. Source shows waits
  for buffered I/O completion / outstanding vnode writes, not evidence
  of producer thread starvation. Trace driver530MB/s write,87MB/s
  read,9.67ms mean write completion; exact kernel-thread cause remains
  outside this two-process trace.
  Important confounder:16.54GB swap-outs and5.71GB swap-ins across
  run,0.84GB swap-outs during trace. Unrelated Python40943 read17.8GB;
  exited before command inspection. Asked user what else was running.
  Prior storage_ceiling_205218 and storage_wal_cache_160503 stalled
  with zero swap-out growth, so swapping is not a universal explanation.
  Counts/windows/source references in run/kernel-analysis.json. Do not
  use this memory-contended capture as a clean physical SSD ceiling.
- [x] Updated capture_kernel.py for next user Terminal run: adds
  kernel_task (PID0) to the two writers so downstream service threads
  are sampled. Requires <=16MiB swap-in+out over10s before workload
  and before triggering; records swap throughout and labels captures
  with >64MiB swapping across collection as contaminated. Captures
  PID/PPID/RSS/executable names at trigger to identify competing jobs.
  Syntax and live swap-counter reader checked; previous trace replay
  exceeds guard at907.6MB swapped in observed trace interval. The
  new privileged capture remains pending user Terminal execution.
- [x] Clean kernel capture storage_kernel_20260908_215802 analyzed.
  Trace17:59:19.577-17:59:29.578,1000 samples/thread, includes
  kernel_task. Capture guard recorded1.44MB swapped; host-aligned
  trace interval0.52MB swap-ins/zero swap-outs,0.52MB/s disk reads.
  Entire182.7s host observation had zero swap-outs. WAL/data maxima
  during trace343/285ms; driver639MB/s writes,5.85ms mean completion.
  WAL writer589/1000 samples sleeping in buf_biowait. NVMe controller
  completion thread0x451 used0.133s CPU,990/1000 samples in workloop
  wait (8 marked runnable),6 in completion handling,2 waiting on VM
  lock. Second controller and ANS2 RTBuddy threads wait throughout.
  This excludes heavy swapping and host completion-thread CPU saturation
  as necessary causes. Downstream completion delay is the leading
  interpretation, not proof of SSD firmware/NAND mechanism or queue
  occupancy; request issue/completion timestamps are absent. No physical
  throughput maximum claimed. Run/kernel-analysis.json and aligned-
  windows.json retain counts, timing, source and explicit limitations.
- [x] Prepared capture_kernel.py --disk-io: same bounded mixed scratch
  workload and swap guards, replaces stack sampling with20s fs_usage
  -w -f diskio after a stall. System-wide disk events retain competing
  traffic; output stays local in disk-io.txt. Purpose: examine individual
  request durations, sizes and overlap, rather than infer queue behavior
  from a sleeping completion thread. Disk-event timing is not a direct
  measurement of internal NAND service time. Local man page verified
  mode, timeout and root requirement; collector syntax checked.
  sudo -n fs_usage -t1 still requires a password, so live capture needs
  user Terminal execution. No request-level conclusion claimed yet.
- [x] Disk request trace storage_diskio_20260908_225705 analyzed:
  all15,278 lines parsed; no lost-event/overflow notice found. Capture
  swap traffic29.61MB passes guard. WAL file2470 disk writes, all1MiB:
  median0.852ms, p9926.719ms, mean5.940ms, max479.752ms;10
  exceed100ms and consume2.841s of14.671s summed request duration.
  Data file10,881 writes, max488.083ms,78 exceed100ms. Other disk
  event bytes68.26MB versus12.44GB benchmark writes (about0.55%).
  Concrete case18:58:27.455430/27.935405: consecutive WAL1MiB
  requests479.288/479.752ms; nearby data1MiB request476.047ms.
  Following WAL requests return to0.24-0.28ms. Long delays are visible
  in disk events themselves, not solely above request submission.
  Inferred overlap is1 WAL/8 data requests, assuming printed timestamps
  are completions; this is NOT physical NVMe queue occupancy. Disk-event
  latency includes queueing and does not identify NAND/firmware cause.
  Together with clean kernel capture, evidence establishes intermittent
  shared disk-completion stalls as the proximal storage bottleneck;
  a universal throughput ceiling or exact firmware mechanism is unproven.
  Run/disk-request-analysis.json and frozen disk-request-analyzer.py
  retain metrics, complete parser coverage and limitations.
- [x] Test data-write queue contribution with matched cached mixed
  storage A/B/A: data ordinary write versus O_DSYNC,90s each,250+600
  MiB/s targets, aligned1MiB requests,8+8GiB files. Stronger data
  synchronization forces each data syscall to wait; it may change both
  queueing and achieved rate. Compare achieved rates and WAL tails;
  do not attribute an improvement to queue depth if data rate also falls.
- [x] Data-sync A/B/A completed90s each: ordinary/synchronous/ordinary
  data writes, runs storage_datasync_231133/231307/231443. WAL rates
  208.2/191.0/97.9MiB/s; data530.1/355.8/353.4MiB/s. WAL maximum
  sampled writes496/234/552ms; intervals with >20ms writes21/58/75.
  All three had zero swap-outs. Synchronous data prevents the same
  asynchronous caller backlog but does not eliminate long shared stalls.
  At similar achieved data rate, middle run has better WAL throughput
  than final control; chronology and changed synchronization semantics
  prevent assigning a precise queueing benefit. No universal cure or
  physical throughput maximum established. Native PG restored, test
  files deleted; settings/results/source in data-sync-comparison.json,
  data-sync-runner.py and each run, indexed in runs.jsonl.
- [x] Same-file load threshold intervention: keep WAL250MiB/s and
  switch data600/100/600/100MiB/s every60s,240s total. Same cached
  aligned1MiB writes,1s data fsync, fixed8+8GiB files; no catch-up.
  Earlier separate-file600/300/600 tests were confounded by chronology.
  This lowers aggregate target850->350MiB/s without recreating files,
  pausing, or changing synchronization. Measure recovery timing and
  tails in both low phases to test whether overload is necessary.
- [x] storage_loadswitch_231837 completed240s, same files throughout:
  data targets600/100/600/100MiB/s; achieved588.1/98.6/365.5/96.7.
  WAL target250 throughout; achieved241.8/243.3/99.7/237.8MiB/s.
  Second high phase had48 sampled WAL intervals>20ms, max595ms;
  final low had6, max536ms. Lower write pressure restores sustained
  WAL throughput on the same files, but does not eliminate isolated
  shared latency spikes: last low-phase WAL284ms spike at226.8s,
  about47s after lowering data load. Thus instantaneous aggregate
  overload alone does not explain every stall. First low phase's long
  writes ended near63s, but final low recovery was not an immediate
  complete cure. Phase boundaries use nearest1s samples; exact rate
  switches and all evidence are retained in run/phase-analysis.json,
  phase-analyzer.py and runner.py, and runs.jsonl. Original PG restored;
  synthetic files removed. No application sustainable maximum claimed.
- [x] NVMe driver telemetry discovered and sampled via libIOReport,
  storage_nvme_232712,180s high mixed load. Tier0-3 BW Scale Factor
  stayed100; all four Throttle Time counters stayed0; elapsed counter
  advances, all power-residency growth is ACTIVE. Mac on AC, Low Power
  Mode off. WAL172.4MiB/s, max520ms,82 intervals>20ms; data492.1MiB/s,
  max484ms. Thus stalls occur with no exposed tier-throttle or power
  transition indication. Channel availability does not guarantee every
  firmware/thermal mechanism is instrumented; no NAND mechanism proven.
  Sample gaps, swap deltas and raw residency units in nvme-analysis.json;
  raw channels in nvme.jsonl, reader/source and runner retained. Normal
  user can read these counters; no additional sudo Terminal capture.
  Asked user whether an external SSD is available for a physical-device
  comparison, the remaining clean fence between this SSD and shared OS
  behavior. Original PG restored; test files removed. No root-cause
  completion or universal maximum claimed.
- [x] Same-file data fsync cadence1/30/1/30s,60s phases, completed
  storage_flushswitch_233813. WAL target250/data600MiB/s, aligned1MiB
  writes, fixed8+8GiB files, PostgreSQL stopped. WAL248.6/179.3/122.5/
  118.0MiB/s; data590.1/486.9/418.3/349.5. WAL max552ms; data2.451s.
  Explicit per-write timings show103 of125 WAL writes>100ms do not
  overlap a data fsync call. Zero swap-outs,17.2MB swap-ins. Explicit
  data fsync overlap is not necessary; returning to1s does not restore
  throughput. Deferred background writeback remains possible, and
  chronological phases cannot assign a precise cadence effect.
  Source, runner, flush-analyzer.py, flush-analysis.json and raw timings
  retained; runs.jsonl indexed. Original PG restored, both files removed.
  Exact lower-driver/controller/firmware mechanism still unproven; a
  second physical device comparison remains the next clean discriminator.
- [x] User has no second SSD; continue same-device discrimination.
  storage_extents_234556,180s, PostgreSQL stopped, aligned1MiB writes,
  WAL250/data600MiB/s targets,8+8GiB fixed files. F_LOG2PHYS_EXT maps
  every30s: all six WAL maps unchanged345 extents; all five fully
  allocated data maps unchanged160 extents. Sampling0.17-0.53ms each.
  WAL133.9/data404.9MiB/s, max544/500ms, zero swap-outs. No data-volume
  snapshots. Progressive sampled extent relocation/fragmentation is not
  necessary; transient changes between samples/metadata work remain possible.
  Added aligned F_NOCACHE1MiB reads targeting10MiB/s during final~86s;
  process disk-read delta838MB.819 reads overlapping workload,p995.94ms,
  two >200ms.290 reads started during long WAL writes,p995.03ms,max219ms;
  286 fit entirely inside a WAL stall, max5.03ms.117 WAL stalls during
  probe, median240ms,max544ms. Thus reads often proceed while writes
  wait; cannot claim every read remains fast or a firmware cause is proven.
  Probe adds load and shares WAL file; no causal throughput comparison.
  Evidence extent-analysis.json/read-analysis.json, frozen analyzers/probe,
  allocation-preflight.json, source, runner, raw counters; runs.jsonl indexed.
  PG restored with durability ON; probes exited, both disposable files gone.
- [x] Observer control storage_observer_235312,240s: expensive host
  sampling off/on/off/on at60s parent-clock boundaries; iostat stopped
  in off phases, ps/proc_pid_rusage/ioreg/vm_stat omitted. One initial
  full host sample, free-space guard and writer logs remain. WAL244.8/
  186.3/120.2/117.1MiB/s in main phases; second off phase53 writes>100ms,
  max496ms. Data591.9/509.5/416.8/411.2MiB/s. Zero swap-outs,20.1MB
  swap-ins. Slowdown persists without expensive observers. Final~2s off
  tail comes from prefill shifting writer/parent clocks; analyzer uses
  actual switches, includes tail separately. Observer overhead not a cure.
  Evidence observer-analysis.json, frozen analyzer/source/runner, raw data;
  indexed in runs.jsonl. Baseline restored and disposable files removed.
  Next storage_datapause_235751 stops+fsyncs data entirely at60/180s,
  resumes at120s, while WAL250MiB/s remains synchronous. This removes
  remaining100MiB/s data traffic from the prior rate reduction control.
- [x] Data pause storage_datapause_235751 completed: data600/0/600/0
  MiB/s60s phases with successful fsync before each pause; WAL remains
  synchronous250MiB/s. WAL248.3/249.9/160.9/249.4MiB/s. Second loaded
  phase41 writes>100ms,max578ms; both paused phases0 writes>100ms.
  Data pause flush1.86/16.20ms. Loaded slowdown has zero swap-outs;
  heavy swapping occurs only after final pause (914.9MB in/643.7MB out).
  Therefore preserve phase-level evidence, not a whole-run no-swap label.
  Clean-preflight repeat storage_datapause_000316 started to confirm.
  Frozen pause-analyzer.py, pause-analysis.json, source/runner/raw evidence;
  indexed in runs.jsonl. PG restored and files removed between runs.
- [ ] Storage-driver event discriminator prepared as capture_kernel.py
  --storage-events:20s ktrace,64MiB buffer, storage-only subclasses
  S0x0302/S0x0520/S0x0601 (installed kdebug.h + ktrace man page).
  This tests availability of driver events beyond fs_usage disk requests;
  no promise the release NVMe driver exports submission/completion events.
  Captures raw NDJSON; event identities/pairing must be validated before
  interpreting durations. No controller service-time result exists yet.
  Native ktrace attempt exits77 requiring root; sudo -n exits1 requiring
  password. This is OS credential availability, not auto-review rejection.
  Existing Terminal collector authenticates locally, bounds storage,
  waits for a quiet-swap stall, restores PostgreSQL and removes test files.
- [x] Clean data-pause repeat storage_datapause_000316 completed240s:
  WAL249.3/249.9/149.1/249.9MiB/s across data600/0/600/0MiB/s phases.
  Second loaded phase49 WAL writes>100ms,max586ms. Final data pause
  fsync1.57ms, then no >100ms WAL writes for60s; max4.27ms among
  samples fully inside pause (first pause max7.74ms). Zero swap-outs
  throughout,1.69MB swap-ins. This repeats the causal recovery when
  concurrent data-write pressure is removed, without previous swap
  contamination. Same files, WAL rate/sync, process counts unchanged.
  Physical driver/SSD mechanism still unproven; pending narrower storage
  event capture requires the user's local sudo authentication. No second
  SSD required. Source/runner/analyzer, pause-analysis.json, raw evidence
  retained and runs.jsonl indexed. Original PG restored, durability ON,
  disposable files gone; syntax/scoped whitespace checks passed.
- [x] User ran storage-events capture storage_events_20260909_001325.
  30,395 JSON events, all filesystem subclass0x0302; no0x0520 IOKit
  storage or0x0601 storage-driver events. Blank final line only; no parse
  failures.15,192 start/done buffer-token pairs,2 unmatched completions/
  9 starts at boundaries; no token reuse while outstanding, pair flags
  equal except DONE.486 unnamed pairs excluded from read/write analysis.
  3,000 matched1MiB WAL writes:median0.286ms,p9923.23ms,max471.79ms,
  14>100ms; one outstanding WAL request. Next issue after long completions
  median0.247ms,max0.351ms. Data max472.09ms,106>100ms; up to10 named
  requests overlap at filesystem layer, not a measured hardware queue.
  413 named reads start and finish inside long WAL requests. Of8.390s
  summed WAL-request excess over4ms pacing budget,42.0% arises from
  >100ms requests;58.0% from shorter delays. This is a request-level
  lower bound, not a complete wall-time decomposition. Do not focus only
  on rare spikes. Trace swap activity4.0MB, guard marks capture usable.
  No deeper driver boundary captured; no NAND/firmware cause established.
  Do not ask for the identical trace again without a new event source.
  storage-event-analysis.json and frozen storage-event-analyzer.py retain
  exact matching, exclusions, timing and limits; runs.jsonl indexed.
- [ ] Steady-state return, user approved: native18 producer+active cursor
  consumer; no delivery/exception consumers, failures, or durability changes.
  Passive-I/O microbenchmark prepared but deferred, not launched.
  Fixed1000-byte payload/no-op successful handler; keep semantics fixed.
  Lever inventory: producer automatic MaxSize/ConcurrencyLimit versus
  explicit ProduceBatch callers (different controls); per-process offered
  rate; producer and consumer process counts; consumer BatchLimit,
  QueueSize, MessageConcurrency, ClaimPollRate; independent producer/
  consumer pool MaxConns/MinConns, query mode/cache, connection lifetime/
  idle/health settings; per-role GOMAXPROCS/GOGC/GOMEMLIMIT; OS thread
  count/CPU/RSS and normal thread safety cap; topic partition size,
  retention and idempotency lifetime; PostgreSQL shared/work/maintenance
  memory, checkpoint/WAL size/timing, bgwriter, autovacuum, WAL compression,
  I/O method/workers/concurrency, connection limits and transport. Persist
  effective settings and observed resource use; vary only relevant knobs.
  Semantic/timeouts/reclaim/lease bounds are correctness constraints,
  not throughput shortcuts. fsync/full_page_writes/synchronous_commit/
  checksums/autovacuum stay ON. CPU availability is shared host capacity;
  GOMAXPROCS is scheduler parallelism, not an OS thread or CPU quota.
  Findings in harness audit: POOL_CONNECTIONS previously producer-only;
  consumer now has CONSUMER_POOL/MIN_POOL and CLAIM_POLL forwarding.
  Paced explicit production added: per-process budget, no catchup after
  reservation falls behind; report achieved rate so missed load cannot pass.
  Smoke scratch_004847:20k/s10s,200k produced/handled once, errors0,
  consumer p99<=71ms; actual independent pools8 verified. Build/vet/
  race check passed. Exact committed-message backlog sampling added for
  paired runs, separately timed; final drain never defines sustainability.
  Begin short80k/120k probes, then extend promising candidates through
  multiple checkpoints under the100GB aggregate retained-space budget.
- [x] Paired screens scratch_005108/005253,80k/120k targets60s,
  four explicit batch callers,batch1000, independent pools8,one producer/
  one consumer,claims4000,queue16000,handlers4.80k achieved79.8k,
  consumer p99<=82ms.120k averaged108.0k and fell below target late,
  p99<=857ms. All11.27m messages handled once,errors0. However exact
  committed-backlog COUNT queries reached1.30s at120k and caused enough
  scanning to perturb the workload; neither run is an accepted tuning
  comparison. Pool waits negligible, heap<73MB, GC CPU<0.6%; larger
  pools/memory have no evidence of benefit at this point.
  Removed repeated COUNT; added100ms atomic progress-only samples,
  keeping expensive runtime statistics at1Hz. Live handler backlog uses
  time-aligned counters with explicit interpolation/accounting limits;
  cursor ID distance remains separate. Full DB/identity checks happen
  after production. Corrected120k screen launched; record comparisons
  only from matching instrumentation. Build/vet/race checks passed.
- [x] Corrected120k screen scratch_005631 (lightweight100ms counters):
  6.634m produced/handled once in60s,errors0; achieved110.5k/s,
  consumer p99<=1671ms.10s production windows after warmup119.7/119.2/
  117.3/89.9/97.6k/s; interpolated handler backlog peaked178k.
  Pool wait producer19ms/consumer3ms total, heap<93MB,GC<0.6%.
  PostgreSQL averaged1.62cores/apps1.78cores; producer waits dominated
  by data writes/extensions. One checkpoint completed; backend relation
  writes1.26GB took24.2 aggregate seconds, extensions8.58GB took8.3s.
  Removing COUNT did not remove the late slowdown. This is a failed
  120k offered-load screen, not a sustainable110k result. Next compare
  eight callers and split producer processes with total pool budget fixed.
- [x] Matched120k/60s process/concurrency screens, pools total8 producer/
  8 consumer: scratch_010409 eight callers/one producer achieved116.2k/s,
  but handler backlog peaked699k and p99<=6027ms. scratch_010554 two
  producers/four callers each/four connections each achieved117.7k/s,
  backlog peaked1.61m and consumer p99 overflowed the10s histogram.
  Every message eventually handled once,errors0; neither passes steady
  throughput. Process split did not fix the consumer stall.
  Gate evidence in scratch_010409/gate-plateau.json: settled/claimed
  remained6261442 for65 samples spanning6.318s. Pending fence moved
  984226->985739 while observer xmin moved984202->985733, passing
  discarded older fences. fresh_claim.go unconditionally replaces the
  pending pair each poll; continuous producers can keep the newest pair
  too young even though older observations would prove progress.
  This is distinct from one long transaction or a busy consumer pool.
  Testing CLAIM_POLL100ms versus20ms as a configuration-only mitigation;
  no production SQL/library change. Observer snapshots are separate from
  claim snapshots, so retain that limitation when interpreting the trace.
- [x] Poll mitigation scratch_010750: same eight callers/one producer,
  target120k/60s, CLAIM_POLL100ms instead of20ms. Achieved114.7k/s,
  consumer p99<=925ms versus6027ms; max interpolated backlog109k
  versus699k. Last10s producer102.1k/consumer102.5k, rather than the
  20ms run's100.4k/30.0k. This supports the pending-fence starvation
  mechanism and a configuration mitigation, not a durable120k result.
  More polling delay raises normal queue latency (median backlog15-24k)
  and may still fail with longer transactions or more consumer pollers.
  CPU-parallelism comparison follows with both roles GOMAXPROCS4.
- [x] Runtime screen scratch_010918: GOMAXPROCS4 for both roles,
  otherwise same120k/60s/poll100ms setup.112.6k/s,p99<=391ms,
  max handler backlog40k. Production40-50s fell79.4k then recovered
  118.0k in50-60s; this is not proof that four beats ten. PG CPU mean
  2.03cores/apps1.81cores; low pool waits/heap<103MB persisted.
  Extending this candidate at100k/s for180s to test repeated writeback.
- [x] Three-minute paired validation scratch_011048: offered100k/s,
  eight explicit callers,batch1000,one producer/one consumer,pools8 each,
  claim4000/queue16000/handlers4,poll100ms,GOMAXPROCS4 each,
  GOGC400/GOMEMLIMIT2GiB; baseline durable native PG18.6 unchanged.
  17.274m produced/handled once,errors0,duplicates0:95,959/s overall,
  consumer p99<=349ms,three completed checkpoints. Time-aligned handler
  backlog maximum34,259,slope-17.8messages/s after10s warmup. Durable
  visible-head minus committed ID distance median62,898,slope-59.7ids/s,
  first/last30s medians64,001/56,046 (IDs are not exact message counts).
  Thus both consumption measures stayed bounded; production missed the
  100k target, with10s dips80.5k and66.6k. This validates an observed
  ~96k paired average for180s, not a fixed100k rate or absolute maximum.
  PG CPU mean1.73cores/apps1.55cores; app heap<91MB,pool waits small.
  Host snapshot:10cores/24GiB,11OS threads per app,existing6GiB swap.
  Late19s sample:zero swap-ins/outs,508,513 compressed and528,345
  decompressed16KiB pages. Memory compression churn is not ruled out;
  this sample cannot establish its causal cost or whole-run swap behavior.
  Native root peak34.1GB,host free minimum106.4GB; aggregate100GB guard
  stayed clear. DB removed,raw evidence/analyses/index retained.
  Next: repeat poll comparison in reverse order, then isolate PG memory
  footprint/compression and producer write/extension costs. More consumer
  processes may worsen the pending-fence churn; test before recommending.
  Retention remains indefinite in these bounded runs, so a full retention
  cycle and repeated finalist runs remain necessary before a maximum claim.
- [x] Reverse-order polling confirmation scratch_012912/013039:
 120k offered60s,eight callers,batch1000,pools8 each,GOMAXPROCS10.
 Poll100ms achieved112.9k/s,p99<=425ms,max handler backlog37,091;
 returning to20ms achieved110.3k/s,p99<=8486ms,max backlog666,988.
 Last10s consumption107.5k/s versus28.4k/s respectively. Together
 with the earlier20->100 comparison, this supports a repeatable
 consumer pending-fence starvation problem and polling mitigation.
 Every message handled once,errors0; no library changes.
 Added optional VM_STATS sampling without pg_buffercache queries and
 retained exact runner source per new run; Python syntax checked.
 Native buffer comparison3GB->6GB->3GB running with all durability
 settings verified and baseline restoration in finally.
- [x] Native memory bracket memory_screen_013212,3GB->6GB->3GB,
 120k offered60s,eight callers/pools8,claim100ms,GOMAXPROCS4:
 scratch_013213116.9k/s,p99<=356ms;013327115.9k/s,p99<=431ms;
 013437105.7k/s,p99<=448ms. No repeatable throughput gain from3GB.
 Whole-host compressed/decompressed pages (16KiB)301k/169k,
 1036k/774k,325k/288k; zero swap-outs,only96/32/20 swap-ins.
 Smaller buffers reduced compression churn but foreground relation
 writes increased3.26GB/52.1s and3.40GB/105.0s versus0.185GB/11.8s
 at6GB (aggregate backend durations). This trades memory pressure for
 writeback; neither "more memory always helps" nor compression alone
 explains the observed rate. All messages handled once,errors0.
 Driver/configs/VM samples/comparison retained;6GB baseline restored
 and durability rechecked. Next unpaced paired120s run keeps6GB,
 poll100ms andGOMAXPROCS4, to measure natural achieved throughput
 without a producer rate ceiling; retain live backlog and drain exclusion.
- [x] Unpaced paired120s screens,baseline6GB/poll100ms/GOMAXPROCS4:
 scratch_013629 eight callers/pool8 produced15.888m,132.3k/s overall,
 consumer p99<=1132ms,max backlog178k,slope-596messages/s.
 Final50s averaged106.7k production/107.0k consumption: the overall
 rate includes a faster opening burst, not a132k steady-state claim.
 scratch_013919 sixteen callers/pool16 produced14.447m,120.3k/s,
 consumer p99 overflowed10s,max backlog1.55m,slope+2599messages/s;
 final40s production averaged75.5k. Extra transactions did not fix
 the later write-limited region and worsened consumption. All messages
 eventually handled once,errors0; retain8 callers/pool8.
 SQL profile in013629 also exposed idempotency cleanup:22 calls,
 zero rows deleted,11.6s execution,1.12m shared block hits. Investigating
 prepared-plan selection rather than disabling maintenance or extending
 cleanup intervals. Scratch exposes per-pool plan_cache_mode and logs
 actual SHOW result; after-load plan probes run in rolled-back transactions
 on the disposable DB. New binary control precedes config comparisons.
 Build/vet/race compile check passed (scratch has no test files).
- [x] Cleanup-plan control scratch_014530,new scratch binary with
  SHOW plan_cache_mode logging,auto for both pools.8.864m/60s,
 147.5k/s overall,p99<=1641ms; not a sustainable rate claim.
 After-load rolled-back EXPLAIN ANALYZE: generic368.7ms/custom249.9ms;
 both scanned all8.864m rows,removed all by the expired-cutoff filter,
 deleted0,and touched56,463blocks. Custom estimated2.95m qualifying
 rows despite an empty expired set. Monitor confirms idempotency table
 had no manual/auto analyze or vacuum during the run. Thus forcing
 custom plans alone is not the demonstrated fix. Testing the existing
 ANALYZE_INTERVAL10s diagnostic while keeping auto plan mode and
 maintenance cadence intact; catalog/index/stats snapshot now retained.
- [x] Statistics diagnostic scratch_014902: periodic broad ANALYZE
  reduced cleanup cost13 calls/144.8ms versus control12/3879.4ms.
  After-load generic/custom plans used created_at index for the expired
  subquery,0.805/0.127ms versus368.7/249.9ms without statistics.
  But production136.8k/s and consumer p99<=6928ms regressed; analysis
  durations1.12/1.48/5.70/11.81s. Last analysis held xid1262647 for
  >11.38s; observer xmin stayed1262647 and consumer settled/claimed
  stayed7599694 while visible messages reached8217000. The broad
  multi-statement maintenance transaction itself blocked the safe gate.
  Do not adopt repeated broad ANALYZE as a throughput fix. Test one
  early ANALYZE of idempotency created_at only; autovacuum and cleanup
  remain enabled. This separates startup statistics from long refreshes.
  Harness ANALYZE log reused the maintenance-process variable, raising
  AttributeError during finally after workload/shutdown/SQL counts were
  complete. Renamed the log variable; recovered normal verification,
  rolled-back plan probes,DB removal,and evidence/index archival.
  Recovery script/reason retained with the run.8.217m handled once,
  message errors0; the harness finalization error is explicitly separate.
- [x] Narrow startup-statistics test scratch_015354: one ANALYZE of
  idempotency_key_4(created_at) at~1s took45ms; normal autovacuum,
  cleanup cadence and auto plan selection retained. After-load generic/
  custom cleanup probes touched3blocks each and took0.632/0.073ms,
  versus56,463blocks and368.7/249.9ms without statistics. This identifies
  and avoids the fresh-table cleanup scan without the long broad analysis
  transaction.8.049m/60s,134.0k/s,p99<=802ms,all handled once/errors0.
  It does not establish a throughput gain over the147.5k control; host
  write variability remains material. Cleanup fell outside the top20 SQL
  entries; add an explicit cleanup-profile capture so cheap work is still
  measured. Normal finalization passed with the log-variable fix.
  Next paired120s test raises claim4000->16000 and queue16000->64000,
  preserving prefetch ratio and8 producer callers/pool8, to reduce claim
  round trips without increasing producer contention.
- [x] Larger claim candidate scratch_015640: claim16000/queue64000,
  eight producer callers/pool8,consumer pool8,handlers4,poll100ms,
  GOMAXPROCS4,one early narrow ANALYZE,otherwise baseline settings.
  14.581m/120s=121.5k/s,p99<=635ms,handler backlogmax98,882,
  slope-98.6messages/s; two completed checkpoints. Final50s averaged
  109.3k production/108.7k consumption,so the overall average still
  includes faster early production. All handled once,errors0.
  Explicit cleanup profile:24calls,0.143251ms total,72block hits,
  zero reads/deletes. Startup statistics remove the scan without
  suppressing cleanup. This reduces waste but is not proof of a
  corresponding overall throughput gain. Candidate validation at
  110k offered for180s launched with identical configuration.
  Next allowed database lever to inspect: WAL file zero initialization;
  previous recycling experiments kept it ON. PG18 documents wal_init_zero
  OFF as skipping prefill work potentially unnecessary on COW filesystems
  (https://www.postgresql.org/docs/18/runtime-config-wal.html).
  Not changed or tested yet; keep fsync/synchronous_commit/checksums/
  full_page_writes intact and compare actual initialization bytes if tried.
- [x] Candidate validation scratch_015954: offered110k/180s,
  claim16000/queue64000,one early narrow ANALYZE,other settings unchanged.
  17.393m handled once,errors0;96,605/s,p99<=679ms,three completed
  checkpoints. Not a throughput improvement over the earlier95,959/s
  three-minute result, and not a sustained110k rate. Handler backlog
  stayed below45,981 (full-window slope+14messages/s); durable committed
  ID distance first/last30s medians75,339/67,993,slope-32.7ids/s.
  Cleanup remained cheap:36calls/0.167ms/107hits/zero reads. No swap-outs,
 176swap-in pages,3.78m compressed/3.44m decompressed16KiB pages;
  whole-host pressure still present. Producer waits now emphasize relation
  extension locks and buffer-content contention; PG~3.14cores/apps2.01.
  Backend extensions22.64GB/46.05aggregate seconds and relation writes
  1.33GB/35.09s; WAL initialization added10.47GB across backends/walwriter.
  This supports investigating producer allocation/write work next, not
  treating the consumer or pool capacity as the only remaining limit.
  Native root peak37.14GB,host free minimum102.82GB; DB removed.
  Current tuning-round comparison under evidence/native18/
  steady_tuning_20260909/comparison.json covers12 runs,121.179m messages,
  including explicit candidate/short-run limits and binary fingerprints.
  No absolute maximum or retention-cycle throughput established.
- [x] WAL zero-fill OFF/ON/OFF screen, native18.6, paired unpaced90s,
  eight callers/batch1000/pools8,claim16000/queue64000,poll100ms,
  GOMAXPROCS4 each and one early narrow ANALYZE unchanged.
  scratch_022305/022502/022652:162.1/122.8/121.3k produced/s;
  36.601m messages all handled once,errors0. Consumer p99 upper bounds
  842/1413/3771ms; last30s production130.3/90.2/88.4k/s. Initial
  OFF advantage did not reproduce; no causal throughput win established.
  Effective setting verified after each restart and captured per run.
  WAL initialization writes516/392/348,bytes516/6,576,668,672/348;
  fsync counts match,aggregate init sync time3.34/5.61/8.52s. PostgreSQL
  18.6 xlog.c confirms OFF writes one byte per new segment and retains
  the subsequent fsync. Thus the lever takes, removes prefill, and still
  suffers stalls; WAL zero-fill is not necessary for the slowdown.
  Backend relation write time14.8/70.0/65.8aggregate seconds; extensions
  28.4/27.4/25.1s. PostgreSQL logical I/O is not physical device traffic.
  No swapouts; chronology, compression and retained WAL files remain
  comparison limits. Normal baseline ON restored and verified afterward.
  Evidence: native18/wal_zero_022305/{comparison.json,analyzer.py,
  driver.py,plan.json,restored.txt}; per-run raw counters/settings retained.
  Repeated OFF run also had a4.50s settled-head plateau at9548000;
  62 consumer commit WALWrite lock samples. Pending fence stayed old
  while observer xmin passed it, unlike the earlier continually replaced
  pending-fence example. Do not infer the same gate diagnosis from p99.
  Details in scratch_022652/longest-settled-plateau.json.
- [x] OFF long validation scratch_022903:110k offered180s,actual
  producer elapsed182.56s including completion of in-flight calls;
  16.2m handled once,errors/duplicates0,88,736/s versus prior ON96,605/s.
  Consumer p99<=1319ms; handler backlog maximum43,465,slope-15.5/s;
  last60s production/consumption74.30/74.35k/s. Two checkpoints completed.
  Failed110k offered load; no sustained-throughput improvement established.
  WAL initialization1101writes/1101bytes,1101fsyncs/9.84aggregate seconds.
  Backend relation writes3.67GB/153.58s and extensions21.00GB/51.58s;
  producer DataFileWrite1471 and WALWrite-lock942 samples across1682
  post-warmup frames. PG1.60cores/apps1.24: CPU is not fully occupied
  while these writes stall. Consumer pool wait37.32aggregate seconds
  warrants tracking despite bounded handler backlog; producer pool87ms.
  No swapouts,664swapin pages; whole-host compression remains a limit
  on attributing differences to one setting. Final native root35.31GB,
  host free105.05GB; storage guards remained active and DB removed.
  Binary SHA256 matches015954 and all three screens. Baseline ON and
  all durability settings restored/verified; no library edits this round.
  Evidence native18/wal_zero_validation_022903/comparison.json.
  Decision: keep baseline WAL initialization ON; missing prefill is not
  enough to remove the bottleneck. Next isolate insert concurrency versus
  transaction batch size with equal aggregate in-flight messages, while
  keeping active consumption and monitoring relation extension/WAL waits.
- [x] Equal in-flight producer batch/concurrency screen completed:
  native18/batch_concurrency_025113,60s unpaced per arm,
  callers x messages/batch8x1000 ->4x2000 ->2x4000 ->8x1000.
  All allow at most8000 messages in outstanding ProduceBatch calls;
  producer/consumer pools stay8,one process each,GOMAXPROCS4 each,
  claim16000/queue64000/poll100ms,one early narrow ANALYZE,baseline
  WAL zero-fill/recycle ON and normal durability. Restart/fresh DB per
  arm,retained WAL files,100GB aggregate guard. Frozen binary unchanged.
  Explicit ProduceBatch calls bypass automatic batch scheduling; callers
  govern simultaneous transactions here. Check actual transaction sizes
  from message xmin after each run,not only the configuration printout.
  Compare database extension/BufferContent/WAL waits and late production/
  consumption/backlog; repeat control to expose chronological changes.
  Runs025114/025229/025348/025510 achieved175.1/138.7/113.1/136.7k/s;
  all33.853m messages handled once,errors/duplicates0. SQL xmin groups
  confirm exactly1000/2000/4000/1000 messages per transaction. Consumer
  p99 upper bounds549/412/589/1226ms; handler backlog maxima96.4/55.3/
  40.1/65.6k,negative slopes in every run. One checkpoint completed each.
  Last30s production164.9/118.5/93.3/95.2k/s: burst-inclusive averages
  do not establish sustained rates. No swapouts; max native root25.30GB,
  minimum host free114.77GB. All settings and binary hash retained.
  Producer mean extension-lock waiters1.143/.243/.037/.807 and
  BufferContent1.092/.176/.005/.774. Fewer transactions sharply reduce
  sampled contention, but no repeatable throughput improvement proven:
  control drift175.1->136.7k is larger than4-vs8 apparent difference.
  Repeated control10-20s190.5k/s with0.16 WALWrite-lock waiters;
  40-50s89.8k/s with3.42 waiters. Initial control same40-50s170.5k/s,
  0.04 WALWrite waiters. This locates the changed waiting, not its deeper
  device cause. Per-window rates/waits in comparison.json, analyzer saved.
  Next4x2000/110k offered180s validation025654 completed below.
- [x] Long4x2000 validation025654:17.724m handled once,errors/duplicates0,
  180.025s,98,453/s average versus earlier8x1000/96,605/s. Consumer
  p99<=973ms; handler backlog max81,708,slope-4.13messages/s. Three
  checkpoints completed; final60s82.74k produced/82.65k consumed per
  second,final30s80.33/80.27k. No stable98.5k or achieved110k claim:
  production slows while consumption keeps up. The pacer skips missed
  slots; this does not prove a110k capacity ceiling under another pacing
  strategy. Actual SQL transactions8862,exactly2000 messages each.
  Extension-lock/BufferContent mean waiters .043/.020: most of the
  earlier insert contention is absent,without a material throughput win.
  Backend relation writes0.581GB/18.99aggregate seconds,extensions22.879GB/
  24.96s,normal WAL writes27.721GB/25.57s. WALWrite lock mean waiters.273.
  PG1.41cores/apps1.32; producer pool wait7ms,consumer18.40aggregate
  seconds. No swapouts,15swap-in pages; whole-host compression persists.
  Native root peak37.20GB,host free minimum102.74GB; scratch DB removed,
  baseline restored/verified. Evidence native18/batch_validation_025653/
  comparison.json includes raw wait counts,per-window rates,CPU/pools,
  actual batch verification,binary fingerprint,and analysis source.
  Five runs this round51.577m messages,all handled once. Interpretation:
  lock contention is tunable but eliminating most of it does not remove
  the late throughput decline. Keep four and eight callers as candidates;
  neither is a demonstrated sustainable winner. Next test smaller batches
  at fixed callers/pool,then validate with unpaced longer windows to
  separate paced-generator behavior from database capacity.
- [x] Smaller-batch screen completed, native18/small_batch_030613:
  eight callers and producer pool8 fixed; explicit batch1000 ->500 ->250
  ->1000,60s unpaced each. Maximum outstanding messages8000/4000/2000/
  8000; only batch size changes. Same frozen binary,consumer pool8,
  claim16000/queue64000/poll100ms,GOMAXPROCS4 each,one early narrow
  ANALYZE,native PG18.6 baseline and durability. Fresh DB/restart per arm,
  active consumption throughout,100GB retained-space guard. Compare
  verified SQL transaction sizes,commit/WAL waiting,backlog and late
  rates. Repeat control before selecting a longer unpaced candidate.
  Runs030613/030728/030847/031012:176.0/142.0/146.7/121.4k/s,
  35.372m messages all handled once,errors/duplicates0; SQL confirms
  exact1000/500/250/1000 messages per transaction. Producer p99 upper
  bounds99/72/33/599ms; consumer456/428/272/1573ms. Handler backlog
  max85.7/50.1/46.5/64.4k,negative slopes throughout. Last30s rates
  164.9/106.8/128.4/103.5k/s; initial control176.0 versus repeated121.4k
  prevents claiming a causal throughput gain from these short screens.
  Smaller250 batch is a latency candidate; contention persists at eight
  callers (BufferContent mean1.108,extension lock.874 for250).
  Normal WAL write operations13,720/16,933/35,083 for first three runs;
  these are PG logical operations,not one physical flush per transaction.
  No swapouts,no identity limit reached,storage guards active. All settings,
  fingerprints,verification,window waits and source in comparison.json.
  Selected250 vs1000 for120s unpaced paired validation,with no pacer;
  monitor identity cap20m and do not score a cap-limited run as full120s.
- [x] Longer unpaced250/1000 comparison031151/031419 completed:
  15.80625m/13.933m handled once,errors/duplicates0;120.019/120.084s,
  no identity cap reached. Average131.7/116.0k/s,consumer p99 upper
  bounds496/1478ms,handler backlog maxima49.9/187.2k with negative
  slopes. But final30s production74.57/86.30k and consumption74.59/
  86.02k: smaller batch is a latency candidate,not a proven higher
  steady-state rate. Both completed two checkpoints. During250 run,
  10-20s176.7k/s had.11 mean WALWrite lock waiters;90-100s66.9k/s
  had3.99. Backend relation writes1.014GB/40.90aggregate seconds,
  extensions20.484GB/31.46s. Producer pool wait.152s,consumer1.322s;
  this unpaced decline does not require the offered-rate limiter.
  No swapouts either run; fixed binary/config,verified batch sizes,
  native root peak36.15GB,minimum host free103.70GB,DBs removed and
  baseline restored. Evidence native18/small_batch_long_031150/
  comparison.json includes per-window rates/waits and all qualifications.
  Six runs this round65.11125m messages,all handled once. Most late
  throughput loss remains downstream WAL/data write waiting; a universal
  SSD ceiling or single deeper driver cause is still not established.
- [x] Small storage-compression sanity probe after benchmarks: temporary
  tables,1000 rows per case,all rolled back; not a throughput test.
  Same1000-byte JSON shape with repeated or deterministic random padding:
  default table target versus toast_tuple_target512/STORAGE MAIN/explicit
  LZ4 both retain1020-byte stored JSONB,uncompressed. Session-local
  default_toast_compression=LZ4 applied to BOTH variants. Positive
  control2500-byte repeated JSON compresses to98bytes in both; every
  value round-trips equal,zero out-of-line TOAST bytes. Thus LZ4 works,
  but changing target does not engage compression for our small rows.
  PG18.6 heapam.c:2334 checks tuple length>TOAST_TUPLE_THRESHOLD before
  calling the TOAST machinery; heaptoast.h defines the separate trigger.
  Source agrees with https://www.postgresql.org/docs/18/storage-toast.html
  (normally about2kB). Do not spend full throughput runs on this setting
  for the fixed1000-byte workload or enlarge messages to game compression.
  Probe source/results retained as toast-probe.py/json in the long study;
  no persistent database setting or library changes made.
- [x] Five-minute achieved-rate/backlog validation completed under
  native18/steady_floor_033104:250-message explicit batches,eight callers,
  pools8,claim16000/queue64000/poll100ms,GOMAXPROCS4 each;80k then100k
  offered for300s each,normal PG/durability and storage guards unchanged.
  Goal is an observed steady operating rate,not another burst leaderboard;
  report missed offered rate and late-window variation explicitly.
  Scratch flag ceiling raised20m->40m only (identity bitset about5MB),
  enabling24m/30m messages without truncating a five-minute run. Prechange
  rebuild SHA256 exactly matches previous a6828be9... frozen binary;
  new binary and source.zip/build.json retained. Build/vet/race compilation
  passed (no scratch package test files). Smoke033105:200k messages
  handled once,errors0,consumer p99<=211ms,40m bound accepted; DB removed.
  No retention or payload semantics changed;100GB aggregate and40GiB
  host-free guards remain in force. No claim of a sustainable maximum.
- [x] Five-minute80k/100k offered runs033121/033710 completed,250-message
  batches,eight callers/pools8,all45.3475m messages handled once,errors0.
  80k achieved78,943/s;consumer p99<=227ms;final60s79,948 produced/
  79,977 consumed per second. Handler backlog max17,611,slope-.31/s;
  committed ID-distance first/last30s medians49,125/46,000,slope-4.27ids/s.
  Seven checkpoints completed. Two10s production windows dipped to
  66.2/65.7k,so this is an observed near79k operating run,not an exact
  80k offered-load pass or a hard minimum rate guarantee.
  100k achieved72,213/s;consumer p99<=1339ms;final60s63,105 produced/
  63,169 consumed per second. Handler backlog max36,750,slope-.38/s;
  committed ID-distance first/last30s55,239/65,250,slope+16.06ids/s.
  Four checkpoints completed; no100k capacity claim. Producer pool waits
  .023/.166s,consumer.038/99.57aggregate seconds; mean PG CPU1.08/1.04
  cores,apps.96/.94. No swapouts,83/36swapin pages. Native peak42.91GB,
  host free minimum96.76GB; DBs removed,baseline restored and verified.
  New mechanism evidence: WAL initialization20/1244 files,0.336/20.871GB,
  init sync.042/12.155aggregate seconds; normal backend WAL write time
  52.02/110.87s. Producer mean WALWrite lock waiters.013/1.956. Higher
  initialization work accompanies fewer completed checkpoints and lower
  throughput; causality/order still needs the lower-rate repeat.
  Whole-device iostat trace began132.8s into80k run; its remaining phase
  averaged495.3 reported MB/s.100k full run averaged455.5,peak961.3.
  These combine all reads/writes and other processes,not a physical SSD
  ceiling or PG-only byte count. Raw trace/timestamps,device analyzer and
  limitations archived. Tail rates now use exact final30/60s intervals,
  avoiding whole-ten-second rounding in short prior-run summaries.
  Follow-up80k/300s repeat034328 completed below.
- [x] Lower-rate repeat034328:23.33425m handled once,errors/duplicates0,
  300.004s,77,780/s achieved;consumer p99<=227ms. Handler backlog max
  34,895,slope+.42messages/s;committed ID-distance first/last30s medians
  47,500/40,000,slope-3.99ids/s. Seven checkpoints completed. Average
  and latency largely recovered after100k offered, but final60s70,220
  produced/70,091 consumed and final30s62,016/61,852 show another late
  dip. Do not call this a guaranteed80k floor or flat sustained rate.
  Last three10s windows63.3/67.1/55.8k;mean producer WALWrite lock
  waiters1.46/1.20/2.37. Only five WAL files initialized,0.084GB/18.7ms
  init sync: large WAL initialization is NOT necessary for these stalls.
  This lowers its priority as a complete cure,while leaving additional
  allocation work as a possible amplifier of the100k run's shortfall.
  Native peak38.87GB,host free minimum100.72GB;no swapouts,20swapin pages.
  Device trace starts51.5s into repeat,mean476.9 reported MB/s across
  reads+writes,all processes;not a device saturation verdict. DB removed,
  baseline restored,both device monitors exited. Three full runs plus
  smoke68.88175m messages verified once; source/binary/commands preserved.
  Combined evidence steady_floor_033104/round-comparison.json retains
  all three runs,exact tail windows,backlog units and limitations.
  Next evidence-led configuration candidate: increase min_wal_size
  (minimum recycled WAL reserve) while retaining max_wal_size/checkpoint
  target and durability,at100k offered. PG18 describes its role in
  preserving reusable files at https://www.postgresql.org/docs/18/wal-configuration.html.
  Measure actual initialization and late rate; do not credit a large
  reserve with a throughput gain unless it reproduces. Not tested yet.
- WAL minimum reserve comparison 2026-09-09, wal_reserve_113412:
  scratch_113412/113747/114125, min_wal_size8GB/2GB/8GB,
  max_wal_size8GB and checkpoint target0.9 fixed. Same frozen binary
  e6246168, 8 callers x250, pools8, runtime4 cores each, 100k offered,
  requested180s each (last actual181.022s), all durability unchanged.
  Actual settings asserted per run; batches verified250 via transaction
  counts. Achieved97,221/83,012/79,596 messages/s; final60s production
  95,144/70,417/66,600 and consumption95,139/70,576/66,729.
  All46,854,000 messages consumed once, zero errors/duplicates.
  Handler backlog maxima26,827/22,936/101,385; slopes-1.87/-3.62/-18.46
  messages/s. Consumer p99 upper bounds242/1050/1212ms; checkpoints
  completed4/3/3. No growing backlog trend at achieved rates; pacer
  skips missed slots, so this is not acceptance of100k offered.
  Starting AND ending WAL inventory512 files/8GiB for every run.
  WAL init8.137/8.171/5.570GB, normal WAL write time30.52/57.55/62.13
  aggregate seconds. Repeat8GB initialized less yet ran slower: no
  repeatable reserve benefit, no evidence that extra initialization
  explains the main late slowdown. Minimum may not bind in this load;
  existing files retained, so do not call this an initialization-free test.
  Producer pool wait0.087/0.156/0.160 aggregate seconds; consumer
  0.021/86.85/53.12s. Late repeat producer WALWrite lock observations
  average about1.6-2.7 concurrent sessions per10s window. Consumer pool
  waits matter to latency but consumer tracks producer output; adding
  producer connections cannot remove waits already inside PostgreSQL.
  PG mean CPU1.50/1.26/1.19 cores, apps1.40/1.19/1.14; no swapouts.
  Native peak34.36GB, minimum host free105.20GB; storage guard remained
  active. Whole-device means563.9/457.7/432.4 reported MB/s; first run
  coverage begins12.6s, other runs near full; all-process traffic, not
  proof of SSD physical saturation. Source, settings, inventory, device
  trace, comparisons and verification retained in study directory.
  Baseline min2GB/max8GB/target0.9 restored; databases removed and
  device monitor stopped. Do not adopt larger minimum on this evidence.
  Next: paired checkpoint target0.5 versus0.9, max8GB/min2GB fixed,
  testing whether earlier checkpoint writes reduce foreground write
  stalls and preserve late rate. Prior producer-only0.5-alone test had
  no active checkpoint;2GB/0.5 long test faded, so no assumed win.
- Paired checkpoint pacing comparison 2026-09-09,
  checkpoint_pacing_114920: scratch_114921/115257/115628,
  checkpoint_completion_target0.5/0.9/0.5, WAL max8GB/min2GB fixed.
  Actual target and durability asserted per run; frozen e6246168 binary,
  same8x250/pools8/runtime4 setup at100k offered,180s requested each
  (last actual181.396s). Achieved98,087/84,643/81,804 messages/s;
  exact final60s production96,315/71,478/67,014 and consumption
  96,347/71,506/67,054. Final30s94,356/70,483/59,758 produced/s.
  All47,731,250 messages consumed once, no errors/duplicates, actual
  batches250. Handler backlog maxima21,605/22,341/23,294, slopes
  -5.00/-10.12/-7.86 messages/s; small negative interpolated gap is
  sampling skew. Consumer p99 upper bounds227/586/972ms. Durable
  committed-ID gaps first/last30 medians57,626/59,858;59,000/46,434;
  58,992/58,627, ID distances not exact counts. No growing backlog
  trend at achieved rates; missed pacing slots are not queued, so no
  claim of100k offered acceptance or established steady-state maximum.
  Completed checkpoints3/3/2; WAL initialization12.231/6.744/11.056GB.
  Normal WAL write time23.916/58.075/46.865 aggregate seconds.
  Client normal relation writes0.121/0.044/0.587GB and4.396/2.440/28.600s;
  extensions22.684/19.575/19.083GB and20.493/21.551/23.675s.
  The0.5 repeat did not reduce foreground writes or restore throughput.
  Late repeat WALWrite lock observations1.03-2.59 average concurrent
  producer sessions per10s window. Producer pool waits0.123/0.145/0.335
  aggregate seconds; consumer0.017/56.48/38.37s. PG CPU1.40/1.22/1.19
  cores and apps1.37/1.18/1.13, no swapouts. Native peak35.71GB,
  minimum host free103.75GB; storage guard active. Device means
  579.9/455.0/453.0 reported MB/s across all host processes; first
  trace starts14.2s, others nearly full; no physical saturation verdict.
  No repeatable benefit: retain target0.9. Both recent configuration
  comparisons have stronger first runs and weaker repeats; do not turn
  chronological drift into a setting win. Fixed WAL size did not hold
  checkpoint count or total I/O work constant. Evidence includes source,
  settings, commands, raw counters, comparisons, device trace and identity
  checks. Baseline restored, scratch DBs removed, monitor stopped.
  Next investigation: one continuous paired workload, high-load conditioning
  followed by lower admitted-rate steps and reversal, without DB restart
  or recreation. Test whether reducing write pressure restores a stable
  rate in the already-loaded system. Plan duration/storage first, preserve
 100GB guard, no-catchup pacing, identity and durable-cursor checks.
- Continuous rate schedule 2026-09-09, continuous_rate_120421:
  scratch-only main.go adds validated elapsed:rate schedule to existing
  no-catchup admission pacer; run.py passes PRODUCER_RATE_SCHEDULE.
  No library edits. Pre-edit rebuild exactly matches e6246168 baseline;
  new frozen binary643f90a3f2fef787df8cea694a0a152dfa859dbaa24ebddffa9866b7d7a6726f.
  go fmt/build/vet pass; race compile passes (no test files). Invalid
  schedules rejected before DB connection. Smoke scratch_120421: four
  3s phases20k/10k/15k/20k match measured rates;195,000 verified once.
  scratch_120440 keeps one producer, consumer and DB running450s:
  100k offered180s,60k90s,80k90s,100k90s. Same8x250/pools8/runtime4,
  default min2GB/max8GB/checkpoint target0.9, durability unchanged.
  Actual rate-change admission boundaries0.000015/180.002274/270.002179/
  360.260446s; final transition delayed by active work, not a restart.
  Phase production96,768/59,667/58,983/67,605 messages/s; consumption
  96,694/59,686/58,925/67,628. Last30s production93,104/59,567/59,883/
  64,783; consumption92,852/59,309/59,632/64,392. After-first5s rates
  and backlog/window metrics retained in rate-phase-analysis.json.
  All34,182,000 long-run messages consumed once (34,377,000 with smoke),
  errors/duplicates zero. Actual450.012s, identity limit not reached,
  batches250 verified. Nine checkpoints completed overall; phase nearest
  1Hz deltas4/2/2/1. Overall handler backlog max23,758,slope-4.30/s;
  committed-ID gap first/last30 medians58,495/58,202 (ID distance, not
  exact message count). Overall consumer p99 upper bound654ms.
  Mean producer WALWrite lock waiters per phase0.029/0.019/1.810/2.172;
  normal WAL write time30.006/5.596/38.859/35.862 aggregate seconds.
  Foreground normal relation write time4.597/4.413/0.104/0.022s:
  later stalls do not require large foreground relation writes.
  Relation extensions remain, as do background/checkpoint writes; do
  not infer the whole storage path is idle. PG phase deltas use nearest
  1Hz samples with exact sample spans saved, not exact boundary snapshots.
  Producer pool wait0.201 aggregate seconds vs consumer39.01s; mean
  CPU PG1.27cores/apps1.11. No swapouts;2661 swapin pages,whole-host
  compression activity persists. Native peak55.98GB plus other guarded
  roots about30.8GB,host free minimum83.32GB. Whole-device mean491.4
  reported MB/s,first sample2.45s; all-process traffic,not physical ceiling.
  Conclusion:60k tracked for90s while later80k and100k targets failed.
  This removes restart/recreation as necessary explanations for the late
  slowdown. It DOES NOT demonstrate recovery: initial100k phase was
  still fast, and there was no final60k step after the measured stall.
  Therefore no claim of causal60k threshold or sustainable maximum.
  Next: continuous100k/60k/100k/60k, placing a low-rate phase AFTER the
  late stall, retaining storage guard and identity checks. Need repeated
  late comparisons before adopting an operating rate. Baseline restored,
  databases removed, monitor stopped; raw phases/build/commands recorded.
- Continuous lower-rate recovery test 2026-09-09,
  rate_recovery_121713/scratch_121713: same frozen643f90a3 binary,
  byte-identical scratch sources and prior archived library source; current
  unrelated library edits excluded from this configuration comparison.
  One continuously running DB/producer/consumer, baseline settings and
  durability,8x250/pools8/runtime4. Offered100k180s/60k90s/100k90s/60k90s.
  Actual phase production97,140/57,537/69,417/44,654 messages/s;
  consumption97,118/57,485/69,417/44,719. Last30s production94,896/
  57,498/66,041/44,448. Excluding first5s:97,188/58,798/68,594/44,618.
  Final60k phase FAILED to recover for90s after observed high-load
  slowdown. Earlier successful60k intervals are not a reliable steady
  operating rate. No evidence of a fixed60k sustainable threshold.
  All32,971,250 messages consumed once,zero errors/duplicates; actual
  producer450.968s,identity bound not reached,batches250 verified.
  Phase monotonic-start origin differs39.5ms from legacy final-elapsed
  inferred origin; rate-phase analyzer uses recorded production_start
  and scheduled boundaries. Admission transitions within2.35ms of
  scheduled180/270/360s. Ten checkpoints completed;phase nearest1Hz
  deltas4/2/2/2,actual sample spans retained. Handler backlog max77,117
  overall (brief early peak),slope-11.67/s;late phase max12,014. Durable
  committed-ID gaps first/last30 medians62,043/33,750 (ID distances,
  not exact counts). Consumer p99 upper bound727ms;consumers tracked
  achieved production throughout,not evidence100k offered was accepted.
  Phase mean producer WALWrite lock waiters0.019/0.100/2.146/1.784.
  Normal WAL write time31.354/9.272/38.695/26.003 aggregate seconds;
  WAL initialized7.315/4.396/2.768/4.580GB. Early/late60k phases each
  complete2 checkpoints and initialize similar WAL bytes, but normal
  WAL write time perMiB grows0.955 to3.402ms (3.56x), with less produced
  data late. Foreground relation write time0.197/0.660/0.0075/0.0309s:
  large foreground relation writes are not necessary for these stalls.
  Relation extension and checkpoint/background work remain; do not
  infer those writes or the entire storage path disappeared.
  PG mean CPU1.30cores/apps1.13,no swapouts,828swapin pages;whole-host
  compression remains. Native peak52.46GB plus other guarded roots about
  30.8GB;minimum host free85.80GB. Device phase means560.8/500.7/422.7/
  392.1 reported MB/s,reads+writes all processes; first coverage14.2s.
  Lower device throughput with higher WAL time is not proof of a fixed
  SSD bandwidth ceiling. Evidence narrows immediate bottleneck to WAL
  write path; accumulated checkpoint/VM/storage work vs underlying
  storage behaviour remains unresolved. Ninety seconds without recovery
  does not prove permanent degradation. Baseline restored,DB removed,
  monitor stopped; phase and overall comparison files include limitations.
  Next: lower40k for120s after the high-load slowdown in the same run,
  probing whether pending write work can drain while consuming, with
 40m identity bound and100GB storage guard. Do not label it the maximum.
- Continuous40k recovery test 2026-09-09,
  rate_recovery40_123305/scratch_123305: frozen643f90a3 binary and
  prior archived source, scratch source identity rechecked. Baseline
  PostgreSQL/durability unchanged, same8x250/pools8/runtime4. One DB
  and producer/consumer run480s:100k180s/60k90s/100k90s/40k120s.
  Produced97,757/58,208/71,818/35,033 messages/s; consumed97,708/
  58,236/71,819/35,066. Final40k phase last60s36,129 produced/36,137
  consumed;last30s35,917/35,992. Actual480.006s,33,502,500 identities
  verified consumed once,zero errors/duplicates,250 batch verified,
  no identity-cap stop. Overall consumer p99 upper bound446ms.
  Handler backlog max24,262 overall and9,914 in40k phase;overall
  slope-15.17/s,late phase-5.50/s. Durable committed-ID first/last30
  medians58,522/22,875,ID distances not exact message counts.
  Eleven checkpoints completed;phase nearest1Hz deltas4/2/2/3.
  Final rate change first admission at360.108702s (108.7ms late,
  captured explicitly);actual sample boundary spans retained.
  Mean producer WALWrite lock waiters0.047/0.007/2.101/1.003.
  Normal WAL write times32.917/10.136/37.084/25.536 aggregate seconds;
  late100k and40k normalized3.368/2.867ms perMiB. Final40k phase
  normal WAL bytes9.339GB. Foreground normal relation write time
  4.420/0.737/0.0424/0.000611s;finalphase onlyone8KiB normalrelation
  write. Extensions5.413GB/4.924s and checkpoint writes still exist.
  Observed equal-completed100ms progress intervals after first5s:
  0.6%/2.5%/21.3%/8.0% of observed phase time. This is counter-based
  evidence of intermittent lost production,not exact syscall stall time;
  report scheduling/bursts affect it. Reproducible analyzer and completion-gaps.json
  retained with raw counters. Lower load helps partially but does not
  clear the shortfall in120s. Three checkpoints do not prove every
  dirty page or pending device request drained. No35k maximum claim.
  PG/apps mean CPU1.11/0.99cores,no swapouts,1126swapin pages;
  whole-host memory compression continues. Native peak52.15GB plus
  other guarded roots about30.8GB,host free minimum86.04GB. Device mean
  465.3 reported MB/s,all-process reads+writes,coverage starts23.9s;
  no physical ceiling verdict. Baseline restored,DB removed,monitor stopped.
  Next: stop descending rate ladder; compare unpaced producer concurrency
  in already-loaded portions of continuous paired runs,using matched
  late-window actual output/backlog. Goal is useful production AND
  consumption between recurring stalls. Existing no-catchup rate targets
  lose slots during pauses,so output below target is not an external
  offered-queue capacity boundary. Keep durability/storage/semantics.
- Unpaced concurrency screen 2026-09-09, unpaced_callers_124920
  and unpaced_repeat_125643: intended8/4/8 concurrent ProduceBatch
  calls in one process, batch250,pool8 fixed,consumer process active,
  native baseline durability/runtime unchanged. Each fresh DB runs180s
  continuously; compare late windows,not one same-DB concurrency switch.
  Frozen643f90a3 binary and scratch source identity rechecked; no new
  library or scratch-runner code. rate=0, no scheduled admissions.
  Valid8-call controls scratch_124920/125643 achieved116,608/120,807
  messages/s overall; final60s69,707/74,328 produced and70,020/74,432
  consumed;final30s63,942/72,537 produced. All42,737,750 valid-run
  messages consumed once,errors/duplicates zero,actual batches250,
  no identity-cap stop. Four checkpoints completed in each control.
  Handler backlog maxima123,338/894,854,slope-111.8/-424.8 messages/s;
  consumer p99 upper bounds1457/5333ms. Backlog drains,but temporary
  large peaks and late rate fade preclude a flat steady-state claim.
  Durable-ID gap maxima252,127/1,002,999,not exact message counts.
  Failed4-call scratch_125300 stopped at61.904s after FOUR10s batch
  attempt timeouts. Driver aborted third planned run;8-call control
  subsequently run separately. No clean4-versus8 concurrency ranking.
  WAL trace: consumer COMMIT pid61739 reported IO WalWrite alongside
  all four producer COMMITs waiting LWLock WALWrite across124 samples
  from08:53:53.269 to08:54:05.569 (12.3s). Transaction age is not exact
  syscall duration; lock-owner role inferred from wait chain,not directly
  observed ownership. Whole-device throughput fell from615-710 reported
  MB/s to mostly0-18MB/s duringpause,then575MB/s at08:54:06;all-process
  counters,not a physical bandwidth ceiling. Host swapout delta0,
  swapin12 over61.78s. Pool contention is downstream here: commits are
  already inside PostgreSQL. Normal consumer work can wait for shared
  WAL I/O and block producing too;this is not archived delivery handling.
  Post-failure SQL:6,161,500 rows/distinct identities,range1..6,161,500;
  consumer seen bitset6,161,500 set bits and no reported duplicates.
  Producer acknowledged6,160,500;1,000 additional messages committed
  despite timed-out calls. Thus do not count failed attempts as uncommitted
  or include this run in failure-free throughput. Raw failure trace/logs,
  wait chain,device window and SQL counts archived; failure indexed in
  runs.jsonl and excluded from comparison's valid_runs. DB removed.
  Independent repeat consumer stall:08:57:51-56 readClaimSnapshot
  active/no PostgreSQL wait,claimed/committed fixed10,100,629 while
  producer head advanced. Snapshot query transaction age reached4.87s;
  subsequent catchup created peak backlog. h.head SQL profile totals
  only998.94ms execution across3667 calls,so do NOT claim5s executor
  scan from this alone. Planning/pre-execution/OS delay unresolved;
  timeline saved in claim-snapshot-stall.json and peak-cursor-frame.json.
  This is a separate consumer-path lead worth pursuing alongside WAL.
  Valid controls PG mean CPU3.39/3.57cores,apps2.02/2.13;no swapouts,
  swapin20/12pages. Native peak44.11GB;host free minimum93.97GB,
  storage guard active. Device partial means730.4/759.7 reported MB/s
  with first samples19.6/32.8s;whole-device aggregate,not PG saturation.
  Both studies restored baseline;all three scratch DBs removed;monitors
  stopped. round-comparison.json keeps successful42.738m separate
  from failed6.162m and records no concurrency winner.
  Next: capture planning versus execution for readClaimSnapshot around
  partition boundaries,retaining WAL/device traces. Need valid4-call
  repeat before ranking concurrency; do not mask failure by timeout
  increase or infer a planner/range-lock cause without measurements.
- Bounded consumer check and return to tuning 2026-09-09:
  Per user direction, no further filesystem/SSD root-cause chase.
  claim_plan_check_131057/scratch_131057 used one120s diagnostic with
  pg_stat_statements.track_planning=on,log_min_duration_statement500ms,
  bind-parameter logging disabled. Temporary runner adds planning and
  execution maxima query; frozen643f90a3 application unchanged. Server
  logs retain Parse/Bind/Execute durations independently per PG18 docs.
  Workload readClaimSnapshot54 plans,total18.576ms,max1.045ms;
  2682 executions,total822.001ms,max81.070ms. Zero slow snapshot log
  entries. All16,824,750 messages verified once,zeroerrors. Earlier5s
  stall NOT reproduced;no query/configuration fix supported. Close this
  lead for now. Diagnostic instrumentation can affect performance;
  mark run purpose diagnostic and exclude throughput ranking. All
  diagnostic settings restored and confirmedoff/-1/-1 in next run.
  Source:postgresql.org/docs/18/pgstatstatements.html and
  postgresql.org/docs/18/runtime-config-logging.html;finding.json and
  diagnostic-server.log retained. No extension of the bounded check.
  Resume unpaced comparison:four-call retry scratch_131414 and nearby
  eight-call control scratch_131901, studiesunpaced_four_retry_131413
  andunpaced_nearby_eight_131901. Same frozenbinary,batch250,pool8,
  consumerpool8,claim16k,queue64k,poll100ms,runtime4each,durability.
  Actual producer calls4/8 (one process); consumer remains running
  throughout each180s fresh-DB run. Four/eight overall118,638/118,869
  messages/s,almost identical. Exact final60s99,230/71,738 produced
  and99,304/71,626 consumed;four-call late production+38.3%. Last30s
  87,172/70,824 produced and87,038/70,521 consumed. Prior eight-call
  control final60s74,328;nearby eight-call reversal strengthens lead.
  Bothnewruns42,755,750 messages verified consumed once,zeroerrors/
  duplicates,250 batches verified,identity bounds not reached. Four
  checkpoints completed perrun. Prior four-call failure scratch_125300
  remains documented/excluded;this repeat does not erase it.
  Four/eight handler backlog max396,247/47,934,slope-162.1/-86.7/s;
  consumer p99 upperbounds1964/908ms. Four-call latency/backlog worse
  in thispair despite higherlateoutput. Four-call output stillfalls
  to87.2k inlast30s: candidate,not established sustainable maximum.
  Mean PGCPU2.52/3.53cores,apps2.01/2.10. Producerpoolwait0.0023/
  0.214aggregate seconds,consumer23.76/10.34. MeanproducerBufferContent
  waiters0.235/0.952,extensionlock0.231/0.735. Finalminute10s windows
  WALWritewaitersfour0.03-0.90,eight1.34-4.31. Lower database contention
  supports testing lowerconcurrency;not evidence of a pool-capacity win.
  No swapouts;swapin12/15pages. Nativepeak40.94/42.32GB,hostfree minima
  97.10/95.68GB;guardactive. Devicepartial means742.2/730.2reportedMB/s,
  all-process reads+writes,notphysicalceiling. Source/runtime commands,
  waits,verification and round-comparison.json retained. Baseline and
  diagnostics restored;allthreeDBs removed;monitors stopped.
  Next: five-minute unpacedfour-call validation,sameconfiguration,
  final120/60/30s windows plus backlog/errors/checkpoints,40midentity
  and100GBguard. Keepstorageinternals and planning investigations closed
  absent new actionable evidence. No timeout/durability changes.
- Five-minute four-call validation 2026-09-09,
  unpaced_four_long_132748/scratch_132748: frozen643f90a3 binary,
  scratch source byte-identity verified,4 concurrent unpacedProduceBatch
  calls,batch250,pools8,runtime4each,claim16k/queue64k/poll100ms,
  native baseline/durability/timeouts unchanged. Actual300.011s;
 29,775,500 messages consumed once,zeroerrors/duplicates,batches250
  verified,identity cap not reached. Overall99,248 messages/s includes
  faster opening work. Exact final120/60/30s produced75,425/71,350/
 69,558,consumed75,458/71,465/69,637. Six checkpoints completed.
  Handler backlog max48,113,mean13,951,slope-21.50 messages/s;
  consumer p99 upper bound579ms,producer26ms. Durable-ID distance
  max218,481,first/last30 medians91,664/52,695,slope-97.1ids/s;not
  exact message counts. No growing backlog trend at achievedrates.
  Three-minute99k late-rate candidate did not persist through5min.
  Do not turn overall99k into sustained99k or call70k the absolute
  ceiling. Earlier38% benefit is short-window evidence;no matched
  five-minute8call control exists,so long-run advantage is unproven.
  MeanCPU PG2.11cores/apps1.66,no swapouts,64swapinpages. Native
  peak54.05GB plus otherguardedrootsabout30.8GB,hostfreemin83.87GB;
  storage guard active. Devicepartialmean668.2reportedMB/s,coverage
 14.3-299.2s,whole-host reads+writes,not physical-ceiling evidence.
  Analyzer now includes exact120s tail alongside60/30s;raw data,
  settings,commands,identity verification and conclusion retained.
  Baseline restored,DB removed,monitor stopped. No additional storage
  or consumer-planning investigation;closed leads remain closed.
  Next configuration test:batch500 versus250 atfourcallers,pool8,
  allother settings fixed. Matched short screens first,then longer
  validation only if late producing/consuming output and backlog improve.
- Four-call batch comparison 2026-09-09, four_batch_compare_133618:
  three fresh native databases, frozen643f90a3 binary, batch500/250/500,
  four concurrent ProduceBatch calls, pools8,180s each; other settings,
  durability, payload, timeouts and storage guard unchanged. Runs
  scratch_133618/134011/134325 consumed20,764,000/12,740,000/15,183,500
  messages once (48,687,500 total), zero errors/duplicates; SQL verified
  actual batches500/250/500, full duration, identity limit not reached.
  Final60s produced90,298/68,224/63,340 messages/s and consumed
  90,253/68,280/63,352. Final30s produced89,833/67,192/50,352;
  final120s produced99,027/67,610/72,514. Batch500's late benefit did
  not repeat; its second run deteriorated through the final windows.
  Retain250 baseline; no longer500 validation justified by this screen.
  No absolute maximum or stable floor established. Consumer p99 upper
  bounds3596/1113/1326ms; handler backlog maxima418,744/39,708/110,802,
  slopes-193.5/-26.1/-38.4 messages/s. Checkpoints completed4/2/3.
  Mean PG CPU2.58/1.74/2.15 cores; app CPU1.98/1.32/1.63. No swapouts.
  Peak native storage43.58/36.71/32.25GB plus other guarded roots;
  host free minima94.47/101.02/105.42GB. Whole-device mean reported
  MB/s737.8(partial)/453.3/503.4; not a physical-ceiling measurement.
  Raw settings, commands, identity checks, comparison and device analysis
  retained. All databases removed, monitor stopped, baseline restored.
  Concurrent naming and consumer-query edits are outside the frozen
  binary's scope. User directs stopping if a needed rebuild meets broken
  code, until they authorize continuing. No library edits made here.
  Next bounded configuration direction: producer pool4 versus8 at fixed
  four callers/batch250, judging late output, backlog and errors; keep
  storage internals closed. This is a proposed next screen, not a result.
- Four-call producer pool comparison 2026-09-09, four_pool_compare_135113:
  pool4/8/4, batch250, four concurrent calls,180s each; consumer pool8,
  claim16k/queue64k/poll100ms, frozen643f90a3 binary and native durable
  baseline unchanged. Runs scratch_135113/135503/135838 verified
  21,146,750/16,619,250/14,187,250 consumed once (51,953,250 total),
  zero errors/duplicates; effective producer pool limits4/8/4 captured
  from running processes, SQL batches250 verified, identity cap not hit.
  Final60s produced101,458/71,842/62,629 and consumed101,505/71,802/
  62,697 messages/s; final30s produced95,282/78,481/56,433. Consumer
  p99 upper bounds253/607/1271ms. Pool4 benefit did not repeat; keep8.
  No sustainable maximum established; chronology and host-state variation
  prevent attributing the first run's advantage to its pool limit.
  Full comparison includes pool waits, backlog, CPU, storage and device
  samples. All databases removed, monitor stopped, baseline restored.
  Next running screen: baseline, claim8k/queue32k, poll200ms, baseline;
  120s each at pool8, four callers/batch250. Each candidate changes only
  its named settings. Retention-window decision requested separately;
  message retention and idempotency expiry must both be accounted for
  before claiming bounded storage. No storage-internals investigation.
- Bounded consumer settings screen 2026-09-09, consumer_settings_140240:
  baseline / claim8k+queue32k / poll200ms / baseline,120s each, fixed
  four callers/batch250 and pools8, frozen643f90a3 binary. Baseline
  claim16k/queue64k/poll100ms; candidates change only named settings.
  Actual processor startup logs confirm settings; source archive retained.
  Runs scratch_140241/140507/140734/141000 consumed12,519,250/
  11,091,500/10,566,750/9,847,000 once, total44,024,500; zero errors
  or duplicates, SQL batches250 verified, identity cap not hit.
  Final60s produced82,431/75,367/69,896/80,496 messages/s; consumed
  82,486/75,280/69,991/80,464. Consumer p99 bounds424/949/3279/1615ms.
  Both candidates below both surrounding baseline late rates. Keep
  claim16k/queue64k/poll100ms; neither candidate warrants longer testing.
  Handler backlog maxima40,237/33,936/237,484/86,312; fitted slopes
  -82.2/-49.7/+28.4/+80.0 messages/s. Full end drain is not proof of
  steady state; short duration and positive slopes prevent a sustainable
  maximum claim. Native peaks32.08/28.11/26.70/26.23GB plus other guarded
  roots; host free minima105.42/109.40/110.73/111.16GB. Guard retained.
  Combined pool and consumer screens verified95,977,750 messages once.
  Both studies restored PG baseline, removed their scratch databases and
  stopped monitors. No library changes or rebuilds; concurrent user work
  preserved. Next: cleanup under load after the user chooses retention
  and duplicate-prevention windows. Asked whether both can be2min for a
  bounded scratch test; unanswered, so existing24h idempotency unchanged.
  A short-window result must not be presented as24h-window capacity.
- Retention validation authorized 2026-09-09: user approved2min message
  retention and2min idempotency-key TTL for scratch only. Keep
  AllowDropPastCommitted=false. Partition size1m gives several expiry
  opportunities in a short run; this differs from prior5m partitions.
  Results apply to these windows, not24h duplicate-prevention capacity.
  Scratch main adds registration TTL flags. Runner captures relation
  allocation, partition presence and key deletion/dead/live statistics;
  verification checks exact consumed identity set1..N after expired rows
  disappear. Surviving-row count/batch statistics no longer represent all
  production. Existing non-retention verification remains unchanged.
  Application source frozen from continuous_rate_120421; isolated build
  in /private/tmp/vulkan-retention-build. Build, vet, go fmt and race
  compile passed (no test files). Identity check rejected missing and
  substituted identities in a small deliberate-negative check. No library
  edits/rebuild from concurrent naming work. Source/binary hashes archived.
  Smoke retention_smoke_141704/scratch_141705:20k/s target180s;
  3,600,000 consumed once,zero errors/duplicates,consumer p99<=213ms.
  First partition dropped175.1s; key deletion observed after120s,
  finalminute deletion rate18,789/s, live-key estimate2.466m. Surviving
  messages2,379,000; exact consumed IDs complete despite cleanup.
  Peak DB4.436GB,hostfree>=123.96GB; native durable baseline restored,
  smoke DB removed. Next running validation:480s at80k/s target,
  same2min windows/1m partitions,40m identity cap and100GB storage guard.
- Retention80k validation stopped2026-09-09: retention_80k_142040 /
  scratch_142040, target80k/s for480s,2min message/key TTL,1m partitions.
  Janitor timed out before expiry (~69s): SweepExpiredPartitions exceeded
  default5s CleanupTimeout. Repeated backoff, then both partition sweep
  and idempotency sweep timeouts by~143s. Producer/consumer counts alone
  would have hidden maintenance failures. Stopped intentionally172.06s;
  final producer13,170,000 with2 cancellation errors caused by stop,
  consumer13,170,000 with0 errors/duplicates,p99<=3908ms. No full-duration
  or full-identity verification; ineligible for failure-free ranking.
  Failed run, logs, samples, source and query evidence archived; own DB
  removed, PG restored, device monitor stopped. No sustainable claim.
  Bounded query check: surviving message_log_4_9, SELECT matching the
  sweep's candidate query with cutoff before its oldest row returned0;
  PK index scan removed1,000,000 rows by filter,224.204ms after load
  stopped,180,846 shared hits and2735 reads. This is idle diagnostic time,
  not claimed in-load execution latency. Captured SQL/EXPLAIN in evidence.
  Actual workload pg_stat_statements: partition0 DELETE15 calls,0 rows,
  7.265s cumulative; partition2 eight calls,0 rows,2.630s; other partitions
  also paid repeated zero-result scans. These completed-statement totals
  omit cancelled work. Confirms avoidable scanning; does not prove this
  is the sole throughput limit or fully explain key-cleanup timeouts.
  Frozen sweep loops every surviving partition, filters created_at while
  ordering by id, and probes even partitions with no expired rows. Raising
  the timeout would not eliminate those scans. Default system manager
  constructs topic janitor with nil config; its5s CleanupTimeout is not
  exposed by the frozen top-level ClientConfig/SystemManagerConfig.
  Recommend reviewing this cleanup query before claiming bounded-storage
  maximum throughput. No production query, index or timeout changes made.
  Successful20k smoke proves cleanup works at light load, not an8min
  sustained ceiling. Larger-run storage stabilization remains unproven.
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
  its gitignore rule and `RESULTS.md`. `.bench/claim` is a statement
  profile, not a benchmark; it stays.
- [ ] Root `.gitignore`: the stale `/.bench/*/driver/driver` and
  `/.bench/scale/projector/projector` rules go with their binaries.
- [x] `concepts/alert-history.mdx` states the cadence measurement without the
  deleted benchmark link. HISTORY and decision 0709 retain their recorded paths.
- [ ] `concepts/reliability-lab.mdx` documents the shipped measurement,
  observer, fingerprint, and `runs.jsonl`; the Proposed chaos run stays
  Proposed.
- [ ] Decision records for what this window settled (the lab as the
  harness, guards as checks, the record and rep shape); HISTORY entry;
  the two ROADMAP items removed; root `_bench-design.md` and
  `_bench-methodology.html` deleted.

## Project rename, topic → stream, website and logo sheet

Owner: a separate session from the benchmark-recording and throughput work
above. Coordinate edits to shared library, example and benchmark files with
that session; do not overwrite its in-flight changes.

Exploration accepted as the planning basis on 2026-09-09:
[RENAME_EXPLORATION.md](../RENAME_EXPLORATION.md). SQLStreams is the locked
product name [0725]. Existing artwork is reference material for the new
logo sheet. `topic` → `stream` is in scope throughout the library and
website. Keep message, producer, consumer, consumer group, binding, cursor,
lease, schedule and system; this rename does not change their semantics.

### 1. Identity and public proposal

- [x] Initial naming screen (2026-09-09): SQLStreams is already used in
  README artwork, but the near-identical SQLStream name is used by a
  [Postgres/MySQL observability product](https://sql-stream.com/) and a
  [Python SQL-query CLI](https://pypi.org/project/sqlstream/). Both expose
  `sqlstream` commands. The user accepted the overlap and locked in
  SQLStreams [0725]; do not reopen the shortlist on that basis.
- [x] Select the display name/capitalization: SQLStreams [0725].
- [x] Settle the repository/module slug, Go
  package, CLI binary, environment prefix, diagnostic prefix, metric prefix,
  default schema and canonical docs origin. Approved technical forms [0727]:
  `agentstax/sqlstreams`, `sqlstreams`, `SQLSTREAMS_`, `SS` plus the existing
  four-digit serial, `sqlstreams.`, and schema `sqlstreams`. Confirm the
  exact external destinations before cutover; the temporary website origin
  is settled below.
- [x] Confirm the database cutover [0726]: all existing databases are
  disposable. Rename baseline DDL and recreate; no data-preserving migration
  or mixed-version support for this cutover. Coordinate any shared reset
  with the benchmark session before stopping its processes or deleting data.
- [x] Keep the generated Cloudflare origin for now [0726]; use the current
  address unless a replacement is needed. No permanent domain selected;
  sqlstreams.io is a candidate. No binaries released before domain purchase.
- [ ] Before binary release: select the permanent domain, update site origin,
  diagnostic docsBaseURL and version-manifest references, and verify links.
- [x] Draft and review the rename proposal: identity, stream vocabulary,
  existing semantics, website and disposable-database cutover. The user
  removed the proposal page after review; do not publish or recreate it [0728].
- [x] User approved the public proposal for implementation on 2026-09-09;
  technical identity recorded in [0727]. Review artifacts stay local;
  the website documents the resulting product behavior [0728].

### 2. Logo sheet and visual identity

- [x] Review existing SQLStreams SVGs and semicolon favicon; draft the local
  [logo sheet](../SQLSTREAMS_LOGO_SHEET.html): existing outlined wordmark,
  horizontal lockup, standalone symbol, light/dark and monochrome variants.
  Three directions: semicolon (recommended), stream S, message log.
  User correction: the semicolon follows the wordmark (`SQLStreams;`),
  never precedes it. Sheet and header previews corrected; standalone
  semicolon is for favicon/avatar use.
  Website color correction: `SQL` and the trailing semicolon are amber;
  `Streams` stays light against the blue header. Desktop/mobile previews updated.
- [x] Show README/header/mobile, 16/32px favicon, repository avatar
  and social-preview placements. Specify colors, typography, clear space,
  minimum sizes and accessible labels. Existing lettering provenance is
  stated; its original typeface/license remains to verify before final export.
  Chromium desktop/mobile review passed: no horizontal overflow at 390px,
  no page errors; all direction controls and draft SVG download work.
- [ ] Review the sheet, then deliver editable vector masters and required
  PNG exports. Inspect rendered artwork; changing SVG labels alone does
  not change outlined lettering. Keep the site's board design unless a
  separate change is agreed.

### 3. Library, storage and operator interfaces

- [ ] Rename topic-derived declarations at their owning roots, then public
  aliases/handles, packages, files, methods, configs, owners, worker names,
  migration scopes, SQL catalog/column/index names, row tags and JSON/log
  attributes. Update current vocabulary rules alongside the implementation.
- [ ] Preserve already-neutral names: message_log_<id> and the other
  per-stream table families, plus __system.metrics/alerts/schedules.
  Rename their topic-qualified Go identifiers. Keep the advisory-lock
  numeric namespace; account for changed resource literals in the cutover.
- [ ] Rename brand/module references across all eight Go modules, workspace
  setup, public entry package, CLI path, version detection and path-aware
  tooling. Coordinate any entry-package relocation as its separately
  scoped ROADMAP item rather than silently bundling it here.
- [ ] Update CLI commands/flags/help/completions, environment names, defaults,
  JSON, recipes, examples, e2e/benchmark fixtures and local configuration
  references. Never rewrite an operator's explicit schema or DSN for branding.
- [ ] Replace the VK prefix across errors, events, metrics and alerts while
  preserving every numeric serial. Update registry validation, explain,
  fix text/placeholders, test codes and documentation links together.
- [ ] Update metric names/units/attributes, stored measurement identity,
  reserved prefixes, OTel meter scope and Prometheus validation. Verify
  exported series and active alert evaluation so silent empty dashboards
  are covered by observed behavior, not just migration prose.

### 4. Website implementation and continuity

- [ ] Update site/repository origins, branding, navigation, titles, prose,
  API samples, topic-related slugs and why-vulkan. Review against the
  website CONVENTIONS and VOICE; shipped pages describe shipped behavior.
- [ ] Update executable documentation: PGlite schemas/SQL, sandbox controls,
  examples, SQL parity tests, diagnostic data/types and generated code data.
  Reuse existing exports/checks; no new rename infrastructure by default.
- [ ] Update error-code pages, internal links, redirects for published old
  routes/codes, canonical URLs, sitemap/robots and version manifests.
  Include frozen-version links to the live manifest and old binaries'
  diagnostic URLs in the continuity review.
- [ ] Replace README/site artwork, favicon and relevant introductory/share
  assets; inspect light/dark and desktop/mobile rendering. Review browser
  storage keys and document any preference/read-tracking reset.
- [ ] Update Cloudflare project/deployment configuration and inspect external
  domain/repository settings as needed. Prepare reviewable artifacts first;
  ask before `just site-deploy`.

### 5. Distribution, verification and close-out

- [ ] Update release workflows, GoReleaser, binary/archive names, nested
  module tag paths, Homebrew/Chocolatey metadata and installation docs;
  verify which external listings actually exist before planning cutover.
- [ ] Per chunk: affected-module build/vet, `go fmt ./...`, targeted race
  tests and directly affected e2e tests. Verify alias closure and convention
  discovery; sabotage affected discovery checks before trusting green.
- [ ] Verify a downstream consumer outside the workspace, version reporting,
  CLI help/JSON/explain, site build, sandbox parity, telemetry and URL/asset
  rendering. Audit remaining old-name matches, preserving historical records
  and the user's writing rather than blindly replacing them.
- [ ] At review-ready: full fresh-DB e2e suite. At release: pinned prior-tag
  compatibility lab against the declared verdict, migration compatibility
  table and HISTORY entry citing e2e outcomes. Keep the old side of the
  compatibility harness exercising the old API.
- [ ] Fold accepted exploration into the fixed record-keeping surface,
  remove the root exploration document and completed TODO/ROADMAP lines
  at close-out. Leave all work uncommitted and report git status.
