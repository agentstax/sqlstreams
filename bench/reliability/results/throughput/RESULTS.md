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
A later two-message regression reproduced a claim skipping an unfinished
producer whose transaction id is at or above snapshot xmax. This confirms
an unsafe visibility bound in the library; see the correction below. The
historical failing runs did not record statement-level transaction ordering,
so their exact interleavings cannot be reconstructed.

## Confirmed empty-claim rollback

A controlled fixture called the real ClaimMessagesWithCursor three times
with visible message ids 1 and 2 while another transaction kept the
visibility proof pending. Each call returned no range. After every call,
pending_head remained 0 and pending_xmax remained NULL, instead of
persisting the observed head 2 and its transaction fence.

In fresh_claim.go, the cursor update writes this state, but the low == high
return bypasses Commit; deferred Rollback discards the update. This breaks
the documented ability to reuse a previous poll's observation under
continuous traffic. Its contribution to measured latency has not been isolated. Committing
the observation fixes this state-loss defect; the repeats below show that
large stalls remain.

The library now commits the cursor transaction before returning an empty
claim and propagates any commit error. The regression proves persistence
and a later successful claim while newer transactions are still open.

## Corrected transaction bound and repeats

The consumer treated snapshot xmax as the next unissued transaction id.
PostgreSQL actually uses latestCompletedXid + 1. An already-running producer
can have an xid at or above that value and own a message below the visible
head. The regression allocates A's xid, then B's xid; B inserts message 1,
A inserts message 2 and commits, and B remains open. Before the fix, the
real claim returned range (0,2] with only message 2.

Both cursor claims and fan-out now allocate an observation transaction id
in the statement that reads the head, and wait for every older transaction
to finish. The observation finishes before the claiming transaction starts.
Caught-up polls still allocate no xid. See
[0714](../../../../docs/decisions/0714-consumer-observations-allocate-transaction-ids.md)
for the proof, PostgreSQL sources, and existing-state limitations.

Two repeats used the original 30s, 32k/s, batch 100, concurrency 4 setup:

| UTC checker start | Committed | Handled | Missing / duplicate | Hold achieved/s | Hold p99 | Backlog slope/s |
| --- | ---: | ---: | --- | ---: | ---: | ---: |
| 02:48:23 | 960,000 | 960,000 | 0 / 0 | 31,870 | 41.94s | +16,336 |
| 02:50:57 | 960,000 | 960,000 | 0 / 0 | 31,937 | 39.71s | +15,233 |

Both pass every delivery check and fail the backlog guard. Neither meets
our latency target. The fixes prevent the reproduced skip; they do not
establish sustainable 32k/s throughput or explain all remaining stalls.
Checker imports share the database with still-draining consumers, so the
post-production tail also includes checker interference.

A final 30s baseline at 16k/s (02:55:08 UTC checker start) passed all
checks: 480,000 committed and handled, no missing or duplicate deliveries,
p99 end-to-end 248.5ms across the full run. This remains a short probe,
not the required sustained validation.

The tested library diff is retained in ignored evidence/consumer-fix/library.patch,
SHA-256 89245d2e7e916b1f6f565565e20bc74488c6c38d8b3cc9da36586bbc71e5a318.
The run fingerprints identify base 03dcb58d146718a8c53a0e34537c28b3037dac9a,
dirty. Four database regressions pass on PostgreSQL 17.10 and 18.4: empty-claim
persistence, claim/fan-out visibility, and no xid allocation while caught up.
The two original claim regressions failed before the fixes. Reclaim and
routing labs pass on isolated PostgreSQL 17.10; root build, targeted consumer
vet/race checks, and the documentation build also pass.

## Storage and verification

At 8k/s, PGDATA including checker imports used 1,105,544 KiB and raw
records 160,672 KiB. The largest observed PGDATA footprint was 4,358,172
KiB; raw records stayed below 650,000 KiB. Host free space stayed above
70 GiB in observed checks. Disposable benchmark volumes were removed
between runs; the retained diagnostic evidence now totals about 545 MiB, including
the corrected repeats. Final host free space is about 77 GiB.

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

Next: diagnose remaining stalls and resume configuration tuning.
Native comparison, stricter fixed-window throughput/backlog judgments,
recording/storage changes, and three 15-minute validation runs remain open.
