---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Sustainable throughput keeps both producer and consumer queues bounded

## Context

The requested maximum is simultaneous durable production and consumption
with no growing backlog. Short producer-only or eventual-drain numbers
cannot establish it. The reliability harness currently supplies explicit
idempotency keys, which bypass automatic batching.

## Decision

Use this Mac, 1 KB messages, one group, a minimal handler, automatic
batching, and default failure-only delivery logging as the initial workload.
Caller-key deduplication, extra groups, and full delivery logging are
separate measurements. Require p99 end-to-end <= 1s initially.

Tune configurations before proposing library/SQL optimizations. Compare
Docker and native Postgres under matched versions/resources. Keep fsync,
synchronous_commit, full_page_writes, durable tables, and autovacuum on.

Start with 10–30s probes, then 1–2m candidates. Validate finalists with
three 15m windows after warmup, extending until checkpoints, vacuum, and
retention cleanup occur. Both producer and consumer queues must stay
bounded during production. Report each group's completions separately.

Budget 20 GiB for all benchmark storage and preserve 40 GiB host free.
Measure growth before fixing retention or starting long runs. Cloud and
hardware purchases are outside this scope.

## Consequences

Measurement corrections precede tuning. Configurations and environment
must accompany results. An eventual drain is a correctness check, not
proof of sustained throughput. Recording and checker storage are part of
the budget and their overhead must be measured.
