# Multi-topic throughput ladder -- first reading

Read with `docs/decisions/0711`. This is the first workload the reliability
lab ran as a benchmark, and it was run to set the method, not to publish a
number: single runs, no rep, thirty-second rungs. Raw records: one line per
run in `results/multitopic-{1,4,16}/runs.jsonl`, the scenario declared in
each line.

## Environment

Apple M4 laptop, OrbStack VM with 10 CPUs and 12.6GB; `postgres:18.4`
capped at 8 CPUs and 8GB with the compose settings (shared_buffers 2560MB,
max_wal_size 4GB, checkpoint_timeout 5min, autovacuum on, fsync on);
producer and each consumer replica capped at 2 CPUs. Storage, measured
with `pg_test_fsync -s 2` on the lab's volume: fdatasync 2233 ops/s
(448 usecs), fsync 2106 ops/s, open_datasync 1049 ops/s.

## Method

Scenarios `multitopic-1`, `-4`, `-16` at time scale 0.25: four rungs of
30s at 2000, 4000, 8000, 16000 messages per second in total, split evenly
over the topics; two groups per topic, each with two instances per
consumer replica claiming in batches of 100; two consumer replicas;
sixteen producer batch workers per topic. `synchronous_commit` on is the
headline; one off run is a labelled diagnostic. A rung "held" when its
backlog slope stayed under 5% of the rate, no produce started more than
100ms behind schedule, and no generator container passed 80% of its cap.

No checkpoint fell inside any window (the server line reads checkpoints
0 on every run), so the maintenance-rhythm rule is not met here; the
full-length rungs are the next run.

## Readings

Achieved rate is the total over topics; latency is produce, scheduled time
to reply; CPU is the Postgres container's median, percent of one core.

| scenario | sync | 2000 | 4000 | 8000 | 16000 |
| --- | --- | --- | --- | --- | --- |
| multitopic-1 | on | held, p50 1.0ms p99 12.9ms, pg 49% | held, p50 1.1ms p99 646ms, pg 72% | gave (slips 4s), 7997/s, p50 518ms, pg 111% | gave, 5173/s, p50 24s, pg 97% |
| multitopic-4 | on | held, p50 1.6ms p99 11.9ms, pg 37% | held, p50 1.3ms p99 128ms, pg 72% | gave (slips 25s), 6319/s, p50 4.2s, pg 138% | gave, 5455/s, p50 25s, pg 135% |
| multitopic-16 | on | held, p50 2.4ms p99 17.0ms, pg 51% | held, p50 1.8ms p99 670ms, pg 82% | gave (slips 12s), 6176/s, p50 1.5s, pg 222% | gave, 6447/s, p50 21s, pg 146% |
| multitopic-4 | off | held, p50 0.8ms p99 3.1ms, pg 54% | held, p50 0.8ms p99 3.0ms, pg 67% | held, 8000/s, p50 0.8ms p99 2.7ms, pg 96% | held, 15979/s, p50 0.6ms p99 502ms, pg 205% |

Server cost was the same in every run: about 1090 WAL bytes and 13.5 WAL
records per committed message with delivery log mode all, 0 deadlocks.
End-to-end p50 at the held rungs sat between 8ms and 43ms.

## Conclusions

1. On this rig the durable ceiling is about 8000 messages per second in
   total, and it is the same at one, four, and sixteen topics. Multi-topic
   contention is not what stops it: the topic count moved nothing.
2. With `synchronous_commit` off every rung held to 16000/s at a produce
   p50 under a millisecond and Postgres at two cores. The ceiling with it
   on is commit latency, not CPU: no container came near its cap and
   Postgres used at most 2.2 of its 8 cores.
3. Open question for the library, not the harness: fdatasync costs 0.45ms
   here and 8000/s in batches of 100 is only 80 commits a second, yet a
   batch took about 200ms with sync on against 100ms off. The wait is not
   the raw fsync rate; sixteen producer workers and sixty-four consumer
   instances all committing against one WAL is the suspect.
4. Two generator settings were needed before the numbers meant anything:
   the pacer's in-flight cap must scale with the rate (a fixed 256 slipped
   first at every rung above 2000/s), and the consumers needed batches of
   100 to drain a ladder in minutes rather than tens of minutes.

## Next

Full-length rungs (two minutes, spanning a checkpoint), three reps, then
rate runs at half the held maximum for the latency spectrum, and a
follow-up on the commit-wait anomaly in conclusion 3.
