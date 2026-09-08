# Throughput exploration — 2026-09-07

No sustainable maximum is established. After the active-consumer correctness
fix, configuration tuning reached a clean 20s hold at 63,629 messages/s,
p99 410.8ms, with no positive backlog slope or scheduling slips. Its startup
phase failed the backlog guard, so the whole run is not a passing result.
The one-minute 48k/s candidate reached p99 984.7ms but backlog grew by
397.6/s; the harness PASS is weaker than the user's no-growth criterion.

The current promising configuration uses two producer processes, each with
batch size 1000 and one batch transaction at a time; one consumer process /
instance with batch 4000, queue 16000, message concurrency four, 20ms idle
polling; pools of 32 connections; five-million-row partitions; PostgreSQL
18.4 with jit=off; GOGC=400, producer GOMEMLIMIT=2GiB / hard memory 3GiB,
consumer GOMEMLIMIT=2GiB, six-CPU allowances per application process.
Database fsync, synchronous_commit, full_page_writes and autovacuum remain
on. The archived delivery consumer is excluded and exception consumers
are suspended. All completed configuration probes since exception-worker suspension
(03:28 UTC onward) have zero message loss, duplicates, handler errors,
reclaims, and recovery counts.

Native comparison, partition rollover, and three 15m maintenance-inclusive
validations remain outstanding. Long validation first needs bounded
recording and retention to remain within 20 GiB. Scripts and overrides
are retained under evidence/tuning-scripts/; each run also saves its
effective compose configuration. Chronological evidence follows.

## Initial workload and environment

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

## Initial recorded runs

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
[0714](../../../../.docs/decisions/0714-consumer-observations-allocate-transaction-ids.md)
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

## Scope correction: archived delivery consumer

The user clarified that deliveryconsumer is archived. None of the throughput
runs invoked it: reliability uses the public consumer, which assembles the
cursor message consumer and its exception consumer. The earlier fan-out
regression and mixed routing lab did exercise the archived path. Its patch
and newly added tests have been removed; those earlier test outcomes and the
saved library.patch describe an intermediate version. The active cursor fix
is unchanged, so the throughput observations remain applicable. [0715]
records this scope correction. Future validation excludes the archived path.

## Configuration tuning with exception workers suspended

These runs set existing exception_consumer worker targets to zero before
production, restarting consumers and verifying no exception instances are
running. Durability, autovacuum, and other maintenance stay enabled.
No library code changes are part of this tuning. Runtime worker settings
and pg_stat_statements profiles are retained in timestamped *-tuning
evidence directories. The scenario declaration records the application knobs.

At 03:28:00 UTC, a 20s probe offered 32k/s with producer batch limit 1000,
producer concurrency 4, consumer batch 1000, queue 4000 per instance,
handler concurrency 1, two instances, pool maximum 32, and 10ms idle polling.
All 640,000 messages were produced and handled without errors or duplicates.
The 14s hold achieved 31,992/s, p99 70.2ms, and zero fitted backlog growth.
The whole-run checker still failed its six-second warmup backlog guard
(+3757/s), so this is a promising hold result, not a passing full run or
a sustainable maximum. The earlier 32k probes had hold p99 near 40s.

At 03:29:45 UTC, the same settings offered 64k/s for 20s. The hold
produced only 38,067/s, with producer CPU at a median 181% against its
200% allowance, six headroom breaches, and 14 schedule slips. Hold
p99 end-to-end was 18.75s. This run hit the generator's resource limit;
it is not evidence of Vulkan's maximum capacity. The next comparison
raises only the producer/consumer CPU allowances from two to six.

At 03:31:27 UTC, six-CPU application allowances raised achieved hold
production to 45,462/s at the same 64k/s offer. All 1.28m messages were
handled with no errors, but nine schedule slips, growing backlog, and
14.14s hold p99 still disqualify it. Median producer CPU was 460%,
consumer 36%, and PostgreSQL 156%. More CPU helped but did not remove
the producer bottleneck.

At 03:34:37 UTC, producer GOGC=400 and GOMEMLIMIT=4GiB (a soft Go
runtime limit), with six-CPU allowances, kept up with 64k/s: achieved
hold 63,983/s, no schedule slips, and median producer CPU 106%.
All 1.28m messages were handled, but consumer backlog grew and hold
p99 was 8.13s. GODEBUG=gctrace=1 was enabled for this diagnostic;
subsequent runs omit that logging. GC tuning is an application runtime
setting; see [Go's GC guide](https://go.dev/doc/gc-guide).

At 03:36:34 UTC, 50ms claim polling did not help the 64k/s workload:
production held 63,928/s, but backlog grew +26,328/s and hold p99 was
8.42s. Profiling showed both cursor-lock queries and payload reads taking
about 9ms per call; the next configuration removes competing claimers
and increases claim/buffer size and handler concurrency.

At 03:39:11 UTC, one consumer instance, batch 4000, queue 16000,
handler concurrency 4, 20ms polling, and GOGC=400 on both roles passed
the harness at 64k/s over 20s. Producer/consumer Go memory limits were
4GiB/2GiB; CPU allowances remained six each. All 1.28m messages were
handled without errors or duplicates. Hold achieved 63,973/s, p99
1.002s, backlog slope +2104/s. The harness allows up to 5% growth, so
its PASS is insufficient for our stricter no-growth/one-second target.
The next run keeps that configuration and raises both GOGC settings
to 1000 for a 30s validation, still below two million messages.

At 03:41:13 UTC, both GOGC=1000 over 30s handled all 1.92m messages.
Hold production was 63,512/s, p99 899.9ms, but backlog slope +3684/s
failed the guard. Median consumer CPU was 81%, producer 126%, PostgreSQL
196%. The backlog series increased in the latter part of the run; a
sub-second handler latency alone is insufficient to accept this rate.
The next test uses two consumer processes, each running one instance
with its own record writer, rather than sharing one process's writer.

At 03:45:06 UTC, two consumer processes did not improve the 30s result:
hold production 62,861/s, p99 1.92s, one schedule slip. Backlog slope
was zero, but latency and generator timing disqualify it. PostgreSQL
reported only eight blocks read and zero checkpoints during production,
so more database cache is not supported by this evidence. Returning to
one process and GOGC=400, the next probe reduces producer batches from
1000 to 256 and raises batch concurrency from four to eight.

At 03:47:55 UTC, producer batches of 256 with concurrency eight
regressed: hold production 55,477/s, p99 2.44s, two schedule slips.
All 1.28m messages were handled without errors. Revert that experiment.

The harness now exposes the existing TopicConfig.PartitionSize setting
in scenario files and prints it in declarations; targeted bench build,
vet, scenario/runner race checks and documentation formatting passed.
The next test uses five-million-row partitions, producer batch 1000 /
concurrency four, one consumer with batch 4000 / queue 16000 / concurrency
four / 20ms polling, GOGC=400 on both roles, and six-CPU allowances.
This tests steady processing without crossing the current partition.
It cannot establish rollover performance.

The tuning runs through 03:47:55 used producer image
sha256:0fc3018fa1a4407c570a998ac035ae59f9ac735465bd7793c4ddc93b5c278c29.
Their workspace fingerprint advanced when the user committed; the active
consumer fix in that image remained the one already tested. A fresh image
was built for the partition-size mapping.

At 03:52:45 UTC, five-million-row partitions alone regressed the 64k/s
run: hold achieved 45,794/s, p99 9.11s, six schedule slips. Profiling
identified PostgreSQL JIT compilation on the message-read query: 804 calls
used 21,003ms total, with 16,120ms in JIT generation/inlining/optimization/
emission. Evidence: evidence/jit-diagnosis/pg-stat-statements.txt.

The next run changes only PostgreSQL jit=off. This changes query execution,
not durability. PostgreSQL documents that compilation overhead can exceed
the savings for short queries: [When to JIT](https://www.postgresql.org/docs/18/jit-decision.html).
The effective compose configuration is saved with each tuning run.

At 03:55:26 UTC, jit=off reduced message-read execution from 21,003ms
(804 calls) to 5,771ms (1,057 calls), but the whole run regressed:
hold production 40,686/s, p99 11.93s, 17 schedule slips. Producer p99
was 11.61s; the database-read improvement did not establish a throughput
improvement. All 1.92m messages were handled with zero handler errors.

At 04:01:18 UTC, additionally setting GOMAXPROCS=2 on producer and
consumer did not solve the producer slowdown: hold 43,541/s, p99 9.57s,
16 schedule slips, backlog slope +2,750/s. All 1.92m messages handled,
zero handler errors. Restore automatic thread selection for the next
probe and increase producer MaxSize from 1000 to 4000.

The partition-setting build uses producer image
sha256:009c04527dfa88033beb2c4ddcad82d04c910a6629f170f0816ab3000f1e0ed7.
At 04:03:41 UTC, producer MaxSize=4000 regressed: hold 35,124/s,
p99 17.10s, 14 schedule slips, producer median CPU 588% against a
600% allowance while PostgreSQL used 49%. All 1.92m messages handled
without handler errors. The next comparison changes only the lab record
volume to tmpfs with a 2 GiB cap; database data and WAL stay on disk.
This isolates recording storage cost, not database durability.

At 04:05:53 UTC, memory-backed records improved the MaxSize=4000 run
to hold 45,067/s, but p99 remained 9.05s with 19 schedule slips.
All 1.92m messages handled, zero handler errors. This does not establish
record storage as the sole bottleneck. The next test returns records to
disk and producer batches to 1000, with two producers at 32k/s each.
Their names and record keys are producer-a / producer-b; each reads a
half-rate scenario, and the checker reads the aggregate 64k/s scenario.
Each producer has a 3 GiB hard memory cap and 2 GiB Go soft limit.
The effective configuration and both scenario files are retained.

At 04:08:33 UTC, two producers handled all 1.92m messages with every
correctness/recovery/failure count zero. Hold achieved 63,370/s of 64k/s,
p99 788.5ms, zero schedule slips/headroom breaches; backlog slope was
+191.5/s. The harness passes, but its 5% slope allowance is looser than
the agreed no-growth criterion. Observed hold backlog fluctuated around
35k–100k rather than rising continuously; longer validation is required.
Reported producer CPU 65.3% is the median across producer containers,
not their summed CPU. This is a promising short probe, not a sustainable
maximum. The next probe changes only aggregate offered rate to 96k/s
and duration to 20s to retain the 1.92m-message storage bound.

At 04:10:50 UTC, two producers offered 96k/s for 20s: hold achieved
74,203/s, p99 5.49s, backlog +3,195/s and five schedule slips.
All 1,920,032 committed messages were handled without errors, duplicates,
reclaims or recovery. This is an overloaded probe, not sustainable 74k/s.
The next run tests 48k/s for 60s with the same configuration. The previous
64k run peaked at 8,031,024 KiB PGDATA + 1,310,528 KiB records (~8.9 GiB);
1.5 times that message count projects ~13.4 GiB active storage, leaving
margin for retained evidence (~0.54 GiB) and benchmark images below the
20 GiB budget. The operational message cap is 3m for this longer run.

At 04:13:30 UTC, the 48k/s one-minute run handled all 2,880,048 messages
with zero correctness/recovery/handler-error counts and no positive hold
backlog slope. Hold achieved 47,596/s, but p99 1.277s and one scheduling
slip fail the agreed criteria. The larger duration exposed a tail the
short probe did not. Query profiles show 64,502 BEGIN calls overall;
producer transactions remain small despite MaxSize=1000. The next probe
changes producer batch concurrency from four to one per process, at an
aggregate 64k/s for 30s, to let batches accumulate and reduce overlapping
producer transactions. It is still the automatic-batching public API.

At 04:16:47 UTC, one batch transaction per producer improved the 64k/s
hold to 63,629/s, p99 410.8ms, no positive backlog slope or scheduling
slips. All 1.92m messages handled, every correctness/recovery/error count
zero. The warm phase failed backlog growth, so the whole run is a fail;
the clean hold is not a whole-run success or sustainable claim. The next
run repeats the 48k/s one-minute candidate with concurrency one.

At 04:19:38 UTC, the one-minute 48k/s repeat with one batch transaction
per producer handled all 2,880,048 messages with zero errors, duplicates,
reclaims or recovery and no scheduling slips. Hold achieved 47,305/s,
p99 984.7ms, but backlog slope +397.6/s fails the user's no-growth
criterion despite the harness PASS. This is not an accepted sustainable
rate. The next short comparison tests concurrency one at aggregate 96k/s.

At 04:23:06 UTC, the 96k/s comparison with one batch transaction per
producer regressed: hold 58,237/s, p99 9.05s, seven scheduling slips,
no positive consumer backlog slope. All 1,920,032 messages handled and
all correctness/recovery/error counts zero. Lower transaction concurrency
helped at 64k/s but did not scale to 96k/s; it is not a universal setting.
Producer p99 was 8.96s, identifying the producing path as the next profiling
target. No sustainable maximum is established. All benchmark containers
and disposable volumes were removed after the run; host free space was
70.8 GiB. Native comparison and long-run storage work remain outstanding.
