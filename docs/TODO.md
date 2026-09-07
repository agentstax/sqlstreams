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

- [ ] `-role observer`: its own compose service, samples at 1 Hz into
  `observer.sample.jsonl`: `pg_stat_wal` (records, fpi, bytes),
  `pg_stat_checkpointer`, `pg_stat_database` (xact_commit, deadlocks,
  blks_hit/read), and per group the cursor lag
  (`max(message_log.id) - committed`). Read-only on the library's tables,
  through `pkg/topic`'s table-name funcs. Starts with the consumer, stops
  after the producer exits.
- [ ] The checker loads `lab.observer_sample` and writes before/after
  deltas into `measure`: WAL bytes, records, and FPI per committed message,
  checkpoints in the window, deadlocks. The raw series stays in the run
  directory.
- [ ] `backlog_bounded` check: seconds in which cursor lag exceeded its
  value one phase earlier by more than the phase's declared rate (the
  scale bench's `diverging` verdict, standardized). Want `0`.
- [ ] `generator_headroom` check: the recipe samples `docker stats` for the
  producer and consumer containers into the results directory; the checker
  counts seconds either container sat above 80% of its CPU cap. Want `0`.
- [ ] Sabotage: a scenario whose declared rate exceeds what the container
  can produce must fail `generator_headroom` or `schedule_kept`, never pass.

### 4. Recording and reps

- [ ] The checker appends the verdict, minus the `phases` array, as one
  line to `results/<scenario>/runs.jsonl`. Un-ignore that file; the
  timestamped directories stay ignored.
- [ ] `just reliability-lab scenario time_scale reps`: the stack is rebuilt
  once, the scenario runs `reps` times fresh (compose down -v between), the
  exit code is the worst verdict.
- [ ] `-role report -scenario <name>`: prints a table from `runs.jsonl`
  grouped by fingerprint identity (library sha, image tag, GUCs), median
  of reps per number, and "no rep" where a scenario ran once. Summaries
  come from this, never by hand.

### 5. Multi-topic scenario shape and the first workload

- [ ] `Scenario.Topic` becomes `Topics []TopicDeclaration`: name, delivery
  log mode, groups, and that topic's producer phases. The printer gains a
  section per topic; the `.scenario` files regenerate and their test diffs.
  Producer and consumer roles run every topic's timeline concurrently.
- [ ] Replicas: `docker compose up --scale consumer=N`; the consumer name
  already carries the hostname, so records stay distinct.
- [ ] `multitopic` scenario: a saturation ladder of stepped phases across
  a topic count axis, groups per topic, high goroutine concurrency, pushed
  until a guard fails. Sustainable max per topic count is the highest
  phase whose guards held; then rate phases at fractions of it for the
  latency spectrum. Windows span at least one checkpoint (the observer's
  `pg_stat_checkpointer` delta proves it).
- [ ] Each cell's limiter named from the observer deltas and `docker
  stats`; the writeup is `results/multitopic/RESULTS.md` citing
  `runs.jsonl` and the durability posture beside every number.

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
