# Throughput exploration — 2026-09-07

No sustainable maximum is established. The highest passing short probe was
16,000 messages/s. At 32,000/s, two runs left committed messages without
handler calls even though the durable cursor passed them.

## Workload and environment

Apple M4, 10 cores, 24 GiB host RAM. OrbStack exposes 10 CPUs and about
11.7 GiB RAM. PostgreSQL 18.4 has the compose 8 CPU / 8 GiB limits;
producer and consumer processes each have a 2 CPU limit. Durability and
autovacuum remain on. The development Postgres container remained running.

One orders topic, one processor group, two consumer instances, failure-only
delivery logging. Each message encodes to 1000 JSON bytes (Postgres jsonb's
text rendering adds five spaces). Handler records receipt without external
calls. Pool MaxConns 16; idle claim poll 10ms. No retention during these
small probes. Automatic batching; no caller-supplied idempotency key.

Each ordinary probe has 10s warmup followed by 20s hold. Rates and latency
below are the hold phase. Existing `held` guards still allow a 5% backlog
slope; none of these short runs meets the agreed sustained-run method.
The reported phase window also includes waiting for in-flight produces to
return, so an overloaded run's rate is not a fixed-wall-clock capacity.

## Recorded runs

Each row is one run, not a repeated estimate. Exact declarations and
fingerprints are in runs.jsonl; raw observer series and verdicts are under
the timestamped directories.

| UTC checker start | Offered/s | Batch limit (both sides) | Producer batch concurrency | Achieved/s | End-to-end p99 | Missing handler calls |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 01:41:52 | 2,000 | 100 | 4 | 2,000 | 37.2ms | 0 |
| 01:53:09 | 4,000 | 100 | 4 | 4,000 | 58.7ms | 0 |
| 01:54:37 | 8,000 | 100 | 4 | 8,000 | 35.0ms | 0 |
| 01:55:56 | 16,000 | 100 | 4 | 15,998 | 29.7ms | 0 |
| 01:57:31 | 32,000 | 100 | 4 | 31,738 | 37.58s | 131 |
| 02:00:19 | 32,000 | 100 | 4 | 31,998 | 42.82s | 57 |
| 02:05:38 | 32,000 | 100 | 1 | 26,235 | 35.81s | 0 |
| 02:08:10 | 32,000 | 1000 | 1 | 31,987 | 6.11s | 0 |

All timestamps are 2026-09-08 UTC (2026-09-07 locally). All four 32k runs
failed the backlog guard. A further 10s SQL-logging diagnostic at 02:10:26
had no missing calls, but logging changed the workload; exclude it from
performance comparisons. Zero missing calls under serial production does
not prove the concurrency defect's cause or a safe general workaround.

## Reproduced discrepancy

Saved run: `evidence/20260908T020110Z/` contains database.dump,
records.tar.gz, and containers.log (ignored raw artifacts).

960,000 produces committed; 959,943 handler calls succeeded. No duplicate
calls, no exception rows, no dead-lettered messages. Group 5 (processor)
on topic 4 (orders) had claimed = committed = settled_head = 960000.
Message 195558 remained in message_log_4, schema version 1, produced at
01:59:52.987567 UTC, but had no handler call. Other examples: 195561,
195563, 195564, 195566. All 57 missing ids span 195558–425849 and six
transaction-start timestamps. Reading the final compressed raw handler
file independently also counted 959,943 calls and none of those examples.

The missing messages are durable but their group has advanced past them.
The exact library cause is not established. Overlapping producer
transactions are implicated by the serial comparison; claim visibility
and cursor advancement need a deterministic concurrency test before a fix
is proposed. No library code was changed during this exploration.

## Storage and verification

At 8k/s, PGDATA including checker imports used 1,105,544 KiB and raw
records 160,672 KiB. The largest observed PGDATA footprint was 4,358,172
KiB; raw records stayed below 650,000 KiB. Host free space stayed above
70 GiB in observed checks. Disposable benchmark volumes were removed
between runs; about 358 MiB of compressed diagnostic evidence was retained.

This harness retains all messages and writes/imports every record. Linear
extrapolation of the 8k probe is about 36 GiB for a 15-minute run at only
8k/s, already above the 20 GiB budget. That is an estimate, not a long-run
measurement. Retention alone cannot bound the ever-growing record/import
files. The long-run recording design must be resolved before validation.

Build, vet, race tests, the dev scenario at 1/6 time scale, and
database-backed fixtures cover the harness
changes. Overlapping topic ids and multiple groups no longer collapse in
latency measurements. Missing cursor progress and unfinished exceptions
fail completion checks without success audit rows. Removing one actual
handler record from the 2k probe failed with exactly one undelivered
message. Imported record tables are analyzed before verification queries.

Next: isolate the delivery discrepancy, then resume configuration tuning.
Native comparison, stricter fixed-window throughput/backlog judgments,
recording/storage changes, and three 15-minute validation runs remain open.
