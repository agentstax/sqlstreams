---
status: superseded
date: 2026-09-07
phase: "pre-v1"
---

# The reliability lab is the benchmark harness, and a benchmark is a scenario

Superseded by [0745](0745-maximum-throughput-uses-the-reliability-lab.md).

## Context

The benchmark-recording design round of 2026-08-22 drafted a shared
Postgres-bound harness: open-loop driver, warmup and window phases,
histogram merge, environment capture, guards, a record writer. The
reliability lab [0687], built afterwards, already has that shape: one
compose stack, roles, a pacer stamping every produce with its scheduled
time, safety checks that assert the run's premise, a verdict record.
Building the drafted package beside it would be a parallel mechanism.

## Decision

- `bench/reliability` is the harness for every Postgres-bound benchmark.
  A benchmark is a scenario: the same stack, roles, records, checker, and
  verdict. No shared driver package, no histogram dependency, no second
  record shape. `go test -bench` with benchstat covers CPU paths.
- Every run measures. Latency and throughput are computed from the record
  rows the lab already writes; a scenario declares only expectations.
- Guards are checks: `backlog_bounded`, `schedule_kept`, and
  `generator_headroom` join the check set with want 0, counted as seconds
  the guard was violated. A throughput number from a run whose safety
  checks failed is not a number.
- The ledger records every produce. Sampling is the fallback if the
  headroom guard shows the ledger itself as the limiter.
- A fourth role, `observer`, samples server counters at 1 Hz into its own
  record file, loaded like the others.
- The compose file's postgres service is the one pinned environment:
  `postgres:18.4`, CPU and memory caps, the settings, PGDATA on a named
  volume. The former `container.sh` copies are retired as their benches
  fold in.
- The verdict carries a fingerprint block: `fingerprint.sh` writes the
  library commit and dirty flag, the image tag, host, and Docker engine
  facts before the run, since the checker's container can see none of
  them; the checker adds its Go version, the server version, and the
  settings read back from the server. `synchronous_commit` lives there
  now rather than as its own verdict field.
- One tracked append-only line per run in `results/<scenario>/runs.jsonl`;
  timestamped run directories stay untracked.
- `Topic` becomes a list of topic declarations; replicas come from
  `docker compose --scale`. Every run is a scenario, never a cell.

## Consequences

- The multi-topic throughput workload and the idle-fleet benchmark are
  scenarios. compaction folds into a keyed scenario; fillfactor,
  trigger_fanout, and the remaining `env.sh` and `container.sh` copies are
  deleted; idempotency, scale, and claim stay as history or profiles.
- A missing fingerprint file is a lab failure (exit 3), never a verdict
  without an environment.
- **Rejected:** a separate harness package; hdrhistogram-go (the record
  rows are the raw samples); a methodology page ahead of the build.
