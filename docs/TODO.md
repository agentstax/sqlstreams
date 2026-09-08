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
  `bench/scratchnative/results/evidence/native18/` (git-ignored), with
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
  `bench/scratchnative/results/runs.jsonl` indexes retained runs across
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
