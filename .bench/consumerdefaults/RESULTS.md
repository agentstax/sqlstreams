# Consumer default measurements

2026-09-07. Recommend `BatchLimit: 4`, `QueueSize` following BatchLimit,
`ClaimPollRate: 500ms`, and `QueueMargin: 15s`. Keep `MessageConcurrency: 1`,
the 30s message timeout, and the remaining defaults. This favors responsive
local development and short production handlers while keeping batching small.
Long handlers and large fleets have explicit tuning recommendations below.

Accepted in [0710](../../.docs/decisions/0710-consumer-defaults-use-small-batches-and-responsive-polling.md).
The [consumer reference](../../.website/src/content/docs/reference/consumer.mdx)
documents the implemented defaults. Measurements below precede that change;
“Current” in the comparison table means the former defaults.
No claim is made that a synthetic suite establishes what fraction of all
production workloads this configuration covers.

## Recommended settings and the decisions behind them

| Setting | Current | Recommendation | Justification |
| --- | --- | --- | --- |
| BatchLimit | 1 | **4** | About 3.7× local no-op throughput; enough amortization to matter, with less reservation and replay exposure than 8 or 16. |
| QueueSize | BatchLimit | **Keep following BatchLimit** | One shallow batch for ordinary independent messages; avoid increasing waiting and payload retention automatically. |
| ClaimPollRate | 5s | **500ms** | About 484ms p95 in controlled quiet arrivals, and 950ms p95 in the repeated 50-group test. The cost is about six claim/retry queries per idle group per second. |
| QueueMargin | 5s | **15s** | Passed the 3s serial-handler case that failed with 10s. A 30s margin had no demonstrated advantage for the selected ordinary-handler baseline and added another 15s to crash recovery. |
| MessageConcurrency | 1 | **Keep 1** | Application handlers do not become concurrent by surprise. Capacity and downstream limits determine an explicit higher setting. |
| Message.Timeout | 30s | **Keep 30s** | No workload evidence justifies globally rejecting legitimate work sooner. Runtime-specific timeouts remain valuable tuning. |
| MessageMax | Message's values | **Keep** | Avoid changing the authority a message has to request longer execution merely to create waiting allowance. |
| RecordMargin / TimeoutGrace | 2s / 100ms | **Keep** | Distinct persistence and cancellation budgets; neither adds queue allowance in the dispatch guard. |
| ShutdownTimeout | derived from ceiling + grace + record margin | **Keep derived** | Still 32.1s with the default ceiling; queue margin does not belong in this in-flight drain budget automatically. |
| InstanceTTL / ConfigRefreshInterval | 30s / 30s | **Keep** | Ownership renewal and declaration propagation, not healthy message pickup latency. |
| BindingRetryInterval | 10s | **Keep** | Applies to declaration conflicts, not ordinary steady consumption. |
| ExceptionInitialBackoff / MaxRangeReclaims | 5s / 3 | **Keep** | Faster polling already improves ready retry drainage. The tests do not justify making failure recovery more aggressive across all workloads. |
| Retry policy, concurrency policy, start position, bindings | existing defaults | **Keep** | Domain and recovery semantics are not changed by this performance recommendation. |
| SlowDispatchThreshold / DisableGracefulShutdown | off / false | **Keep** | No workload-independent slow-handler threshold was established; normal lifecycle cancellation remains required. |

These choices prioritize a subsecond healthy pickup experience over saving
three more queries per second by choosing a 1s poll. They favor a modest
batch over the maximum no-op benchmark result. Accepting an approximately
47s default crash lease is a product tradeoff, not a mathematically optimal
constant or an externally established standard.

## Research: contributing factors

| Factor | Research and Vulkan consequence |
| --- | --- |
| Human feedback latency | NN/G describes roughly 1s as maintaining a person's flow and 0.1s as feeling instantaneous. Applying that to a local developer's produce/observe loop is an inference, not a queue standard. A five-second idle sleep spends too much of that feedback budget before the handler starts. |
| Batching and fixed database cost | Kafka caches network fetches separately from application polling; RabbitMQ warns that prefetch 1 can constrain throughput. Vulkan's fresh path performs a snapshot read and a multi-statement transaction, then a separate outcome transaction. Amortizing that cost matters even on localhost. |
| Capacity, fairness, and duration tails | River claims only free execution capacity; pg-boss defaults its batch size to 1; Celery recommends shallow prefetch for long work and separate capacity for mixed runtimes. No source establishes a universally correct batch number. Larger reservations can leave a faster instance idle. |
| Lease coverage | SQS distinguishes processing time, reserved-message lifetime, and renewal. AWS's Lambda/SQS advice explicitly includes batch processing and waiting in lease sizing; its six-times multiplier is specific to Lambda throttling/retries and is not a Vulkan formula. |
| Notification versus sleep | River's notifications and SQS long polling can wake on available data. Vulkan's empty-claim sleep cannot. A 500ms polling period remains a tradeoff with query frequency, not an immediate-notification promise. |
| Independent bottlenecks | The snapshot proof can delay a fresh head; the committed-cursor advancer has a 1s default period; ordered delivery across ranges can depend on that cursor. Faster ClaimPollRate does not remove those gates. |
| Memory and payload size | Queue depth bounds ordinary queued entries, not bytes. Ordered same-key chain members are retained outside the queue. Large payloads and multiple instances multiply retained data. |
| Outcomes and recovery | Handler success is not the same timestamp as range settlement or the committed cursor's advance. Exception processing has its own serial loop, despite sharing BatchLimit and ClaimPollRate with fresh delivery. |

Primary sources, consulted 2026-09-07:
[NN/G response-time research](https://www.nngroup.com/articles/website-response-times/),
[River fetch implementation](https://raw.githubusercontent.com/riverqueue/river/master/producer.go),
[River default constants](https://pkg.go.dev/github.com/riverqueue/river),
[pg-boss worker options](https://pgboss.io/api/workers),
[Celery prefetch tuning](https://docs.celeryq.dev/en/latest/userguide/optimizing.html?highlight=prefetch),
[Kafka consumer configuration](https://kafka.apache.org/42/configuration/consumer-configs/),
[RabbitMQ prefetch/throughput guidance](https://www.rabbitmq.com/docs/confirms),
[SQS lease guidance](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html),
[Lambda/SQS batch and timeout guidance](https://docs.aws.amazon.com/lambda/latest/dg/services-sqs-configure.html).

## Measurements

Environment: root HEAD `e1527ebace6e70fd348807f287ac14fa371d264d`, Go 1.27.0
on darwin/arm64, local container PostgreSQL 17.10 on aarch64. Public Vulkan
API, unchanged library, manager enabled, pool capacity 25 in the new suite.
Each experiment created and removed its own schema. There were 84 experiment
schemas including pilots; every cleanup reported success. Earlier tuning
experiments used separate schemas and are linked below.

Some independent correctness experiments ran together on the same server.
Burst trials were sequential, with other low-volume experiments active for
parts of the run. Treat the rates as local comparative measurements, not
isolated hardware ceilings. The final 50-group repetition ran after the
other suites finished.

For ordinary completed probes, all unique messages succeeded, the claimed
cursor reached the target, no claim leases remained, and every exception
status count was zero BEFORE cancellation. This avoids counting shutdown
partial settlement as a completed workload. The periodically advanced
committed cursor was not an additional completion condition. No-op rates
measure first handler entry through last completion; final settlement was
verified separately.

### Quiet arrival latency and idle queries

Twenty arrivals per polling value, sampling ten phases of the poll period
twice. Each arrival follows a caught-up consumer. Time is from just before
Produce to handler entry, so produce latency is included. These controlled
phase samples characterize polling; their p95 is not a production SLO.

| ClaimPollRate | Measured p50 | Measured p95 | Measured maximum | Steady claim/retry queries per idle group/s |
| --- | ---: | ---: | ---: | ---: |
| 100ms | 57.8ms | 108.3ms | 109.7ms | about 30 |
| 250ms | 142.2ms | 242.1ms | 243.1ms | about 12 |
| **500ms** | **283.5ms** | **484.0ms** | **485.9ms** | **about 6** |
| 1s | 558.2ms | 958.4ms | 962.4ms | about 3 |
| 5s | 2753.8ms | 4754.5ms | 4762.3ms | about 0.6 |

One fresh snapshot query plus exception Kill and Claim per period gives the
steady estimate `3 / poll_seconds`. Targeted tracing confirmed the count:
500ms produced 21 calls of each over 10.5s. At 5s the short measurement window
included three of each; boundary effects matter at that sample length.
Manager, cursor-advancer, heartbeat, and other maintenance queries are extra.
The first tracing pilot counted built-in consumers too; its query totals
were excluded and the complete latency/query test was repeated with an exact
physical-table filter for the tested topic.

500ms leaves more latency headroom than 1s without paying twice the idle
query rate for the additional roughly quarter-second improvement at 250ms.
100ms is an explicit low-latency tuning choice when thirty idle queries per
second per consumer group is acceptable. A notification design is a separate
roadmap item if many groups need near-instant pickup.

### Fast bursts and artificial transport delay

Three trials per batch. Concurrency 1, QueueSize equal to BatchLimit, polling
500ms, margin 15s; 2,000 preproduced small unkeyed messages per local trial.
The delayed-transport trials used 400 messages and added a 1ms sleep to each
socket Read and Write. SELECT 1 measured about 2.5ms instead of 0.06–0.07ms.
This is a controlled sensitivity experiment, not a physical network RTT test.

| Batch | Median local messages/s | Relative to batch 1 | Median with delayed socket I/O |
| --- | ---: | ---: | ---: |
| 1 | 891 | 1.00× | 66 |
| **4** | **3,328** | **3.73×** | **263** |
| 8 | 5,814 | 6.52× | 527 |
| 16 | 9,899 | 11.11× | 991 |

Batch 4 is a compromise, not the throughput winner. Two 200-message probes
with 64KiB padding also completed without repeats at batches 4 and 8; these
establish a payload-shape check, not a memory-capacity benchmark.

### Handler duration, ordering, and margin

| Workload | Configuration | Result |
| --- | --- | --- |
| Twelve 2s handlers | B4/Q4/C1, margin 10s or 15s | All 12 succeeded and settled, no repeats. |
| Twelve 2s handlers | B8/Q8/C1, margin 10s / 15s | Only 9 / 11 unique successes by the 29s observation limit. |
| Twelve 2s handlers | B8/Q8/C1, margin 30s | All 12 succeeded and settled. |
| Eight 3s handlers | B4/Q4/C1, margin 10s | Seven unique successes by 28s; one range still open. |
| Eight 3s handlers | **B4/Q4/C1, margin 15s** | **All eight settled at 24.09s, no repeats.** |
| Twenty messages: four 4s handlers among sixteen 50ms handlers | B4 or B8, C1, margin 15s | All twenty succeeded and settled without repeats. |
| Twenty-four 500ms handlers, one ordered key | B4 or B8, C1, margin 15s | All completed without repeated successful handlers; settlement took about 27s / 25s, illustrating additional ordered-delivery gates. |
| Three 12s handlers | B4/Q4/C1, margin 15s | Only two succeeded by 42s. This is outside the baseline's tested comfortable duration range. |
| Three 12s handlers | B1/Q1/C1, margin 35s | All three settled at 36.06s, no repeats. |

The incomplete cases stopped at an observation deadline; they are not
claims of permanent message loss. Successful handler executions were counted
separately from unique messages. An ordered chain does not become parallel
when MessageConcurrency rises; an earlier batch-8/concurrency-4 test on one
ordered key still stalled with only three unique successes at margin 5s.

Concrete successful case: `probe`, topic ID 4 in schema
`defaults_three_seconds_m15_1788819142141102000`, processed IDs 1–8 with B4,
Q4, C1, and 15s margin. A full queued batch can wait behind roughly four
3s dispatches, consuming about 12s of the allowance. With default timeout
and ceiling both 30s, the actual start guard leaves 15s for claim age rather
than 10s. Database overhead and runtime variance still need room; this is
not a guarantee for every handler shorter than 30s.

### Two instances with unequal handler speeds

Thirty-two messages; one instance took 500ms/message and the other 50ms.
Three repeated trials per batch, instances started before production. All
messages completed without repeated successes.

| Batch | Slow instance's message counts | Median total settlement |
| --- | --- | ---: |
| 1 | 4, 4, 4 | 2.06s |
| **4** | **7, 7, 7** | **3.57s** |
| 8 | 10, 9, 9 | 4.56s |

Larger reservations leave more of a finite burst attached to the slower
instance. Batch 1 wins this workload; batch 4 buys throughput amortization
at a measurable fairness/tail cost. The first parallel pilot had additional
startup variability and was not used to rank the candidates.

### Retry drainage

Eight messages each deliberately failed their first attempt, then succeeded.
Every completed case recorded eight successes from sixteen handler attempts.
The default 5s ExceptionInitialBackoff remained in place.

At B1/P5s the last success was roughly 44s after the first handler started;
at B4/P500ms it was roughly 5.77s. This is a small, initially failing burst,
not a sustained failure-capacity benchmark. The exception loop's ideal
short-handler tick envelope changes from `1 / 5s = 0.2/s` to
`4 / 0.5s = 8/s`; serial handler and recording time can lower it. The initial
backoff still prevents retries from becoming immediate when polling speeds up.
High-throughput workloads with sustained failures must size this path too.

### Abrupt process death

The experiment's child process exited immediately on entering its first
handler. A replacement consumed the same group. Polling was fixed at 500ms
in every case to isolate lease duration, including the current-margin case.
Every run eventually settled all eight messages.

| Batch / QueueMargin | Time to start reclaimed message 1 | Total settlement |
| --- | ---: | ---: |
| 1 / 5s | 37.19s | 37.21s |
| 4 / 10s | 42.15s | 42.16s |
| **4 / 15s** | **47.18s** | **47.22s** |
| 4 / 30s | 62.22s | 62.27s |

The recommended margin adds ten seconds to the lease, not to ordinary handler
execution or the default shutdown drain. Crash timing and a later poll can
add different residual waits in a real deployment.

### Fifty groups sharing one pool

Fifty consumer groups on one topic, all ordinary managers enabled, one pool
capped at 25 connections. After warm-up, count ten seconds of idle traffic,
then produce five arrivals at different phases. Each arrival goes to all
fifty groups: 250 correlated observations per polling value. This final
repetition ran after the other suites finished.

| Poll | Own claim/retry queries/s | Wake p50 | Wake p95 | Wake maximum | Pool exhaustion wait, summed over idle window |
| --- | ---: | ---: | ---: | ---: | ---: |
| **500ms** | **300.0** | **270ms** | **950ms** | **1538ms** | **10.9ms across 21 waits** |
| 1s | 151.1 | 598ms | 1849ms | 2851ms | 0 |

Neither run cancelled a pool acquisition. Including existing manager work,
there were 19,286 / 18,565 pool acquisitions over the respective ten-second
idle windows. Pool acquisitions are not SQL statement counts, and summed
client query/wait durations are not server CPU measurements. The claim-rate
change does not explain all upkeep cost; the existing roadmap's idle-worker
traffic item remains relevant.

The maximum exceeded one polling period even with little pool waiting.
Code inspection identifies the snapshot proof and concurrent transactions
as contributing mechanisms; this test did not attribute every delayed
message to a specific query. Recommend 500ms as a responsive baseline, not
as a hard 500ms latency guarantee.

## Workload-specific recommendations

| Workload | Recommendation | Cost or limit |
| --- | --- | --- |
| Ordinary short handlers, local development | Default B4, Q4, C1, P500ms, margin 15s; keep timeout 30s. | Six idle claim/retry queries/s/group; a 47.1s lease; slow handlers still need sizing. |
| Long serial handlers | B1/Q1; set the handler timeout from legitimate runtime and allow at least one preceding dispatch plus DB slack in QueueMargin. Tested 30s timeout / 35s margin with 12s handlers. | Less batching; this tested profile has a 67.1s lease. A realistic shorter timeout can reduce recovery delay. |
| Sustained very fast work | Explicitly raise BatchLimit to 8 or 16 and keep QueueSize shallow; choose concurrency from CPU and downstream capacity. | Higher no-op throughput, more reservation and replay exposure. The fastest benchmark is not a reason to make every handler concurrent. |
| Many quiet groups | Start with P1s if the additional pickup delay is acceptable; use measured query and pool pressure to decide. | Halves claim/retry queries versus P500ms. The repeated fleet test's p95 rose to about 1.85s. |

The implementation adds resolved queue, concurrency, polling, and lease
budgets to the starting log. VK0105 warns once per locally tracked range
when a queued message cannot start safely within its lease. Shared warning
suppression can collapse further ranges within a minute. No adaptive
claiming mechanism or message-range renewal was added.

## Reproduction and limitations

Harness files and JSON/raw logs live in [/tmp/vulkan-default-lab](/tmp/vulkan-default-lab).
Build from the repository so its module and toolchain apply:

```sh
go build -o /tmp/vulkan-default-lab/run /tmp/vulkan-default-lab/main.go /tmp/vulkan-default-lab/crash.go /tmp/vulkan-default-lab/scale.go
/tmp/vulkan-default-lab/run idle
/tmp/vulkan-default-lab/run burst
/tmp/vulkan-default-lab/run workload
/tmp/vulkan-default-lab/run focused
/tmp/vulkan-default-lab/run retry
/tmp/vulkan-default-lab/run crash
/tmp/vulkan-default-lab/run scale
```

The final source includes the corrected topic-specific tracer and five-burst
fleet test. Authoritative logs are `idle-targeted.log`, `burst.log`,
`workload.log`, `focused.log`, `retry.log`, `crash.log`, and `scale-repeat.log`.
`idle.log` and `scale.log` are retained pilots. All 84 new experiment schemas
were removed; the existing database installation was never reset.

The earlier race-instrumented tuning harnesses and logs remain at
[/tmp/vulkan-batch-levers.go](/tmp/vulkan-batch-levers.go),
[/tmp/vulkan-batch-levers.log](/tmp/vulkan-batch-levers.log),
[/tmp/vulkan-batch-combined.go](/tmp/vulkan-batch-combined.go), and
[/tmp/vulkan-batch-combined.log](/tmp/vulkan-batch-combined.log).
Those runs reported no races. The new timing suite was built without race
instrumentation to avoid adding its overhead to the latency comparisons.

These are synthetic workloads on one local database, with small repetition
counts for most scenarios. There is no server CPU/WAL/memory saturation study,
real remote-network deployment, broad payload distribution, or statistical
survey of production handler durations. The recommendation is a justified
starting point with measured boundaries, not a universal optimum.

## Implementation verification

After applying [0710], an additional isolated-schema probe passed eight
three-second handlers using zero-valued session options. All eight ran once,
with peak concurrency one; all range outcomes were recorded in 24.08s,
with no open leases or exceptions before cancellation. Schema cleanup passed.
The printed zero option fields in `/tmp/consumer-default-smoke.log` are inputs,
not resolved values. This run verifies the shipped default resolution path.

The group-config, ordered, and shutdown-truncation e2e tests passed with only their
client/datastore schema settings adapted to temporary namespaces. Each schema
was removed. The original development-schema attempts encountered a missing
worker_instance_log table; no reset was performed. Affected builds/race tests,
the concurrent stale-range warning test, conventions, site prose checks,
and the site build passed. This is a targeted checkpoint, not a full e2e test suite.

## Detailed control map

**Existing controls: what each actually changes**

Let M be the claim-time `MessageMax.Timeout`, T the individual message's
resolved timeout, G `TimeoutGrace`, R `RecordMargin`, and Qm `QueueMargin`.
The two real expressions are:

```text
lease expiry = claim transaction time + M + G + Qm + R
start allowed when now + T + G + R <= lease expiry
therefore: age since claim transaction <= Qm + M - T
```

Clock differences and claim/transfer overhead consume some of that allowance.
The guard reserves the message's permitted duration, not its eventual observed
duration. Raising both T and M together does not add queue allowance. Lowering
both does not add it either, but shortens crash recovery. Lowering T while
holding M fixed does add allowance. Raising M alone also adds allowance, while
authorizing messages to request a longer run and extending the lease/drain
budgets. QueueMargin is the direct control when waiting is the actual concern.

| Lever available today | What it can solve | Constraint or cost |
| --- | --- | --- |
| Smaller BatchLimit | Less work shares a lease and settlement; fewer successes exposed to whole-range replay. | More claim/commit transactions. Also reduces exception batch size. Does not resize an already-created range on reclaim. |
| Smaller QueueSize | Less ahead-of-handler waiting, less payload retention, less work reserved from other instances. | Must remain >= BatchLimit; setting 0 selects the default, not no buffering. At batch 4, queue 1 is rejected. Ordered chain members live outside the queue. |
| Larger MessageConcurrency | Less waiting and faster completion for independent messages. | More application/DB resource use; one ordered key stays serial. Governs the fresh-message runner, not exception processing. |
| More consumer instances | More aggregate capacity and claimers, potentially on more machines. | Does not shorten an already-claimed local batch's serial wait. Requires actual compute/DB capacity; instances compete for ranges, and a small backlog can already be reserved elsewhere. |
| Message.Timeout, per-message requested Timeout, MessageMin/MessageMax.Timeout | Bound the handler's run and reserve enough lease for it. A realistic timeout can bound tail occupancy; a lower T under a fixed M adds queue allowance. | Defaults fill and bounds clamp the request. A timeout below legitimate runtime turns successful work into failures/retries. Raising T and M together will not cure excessive queue wait. |
| QueueMargin | Directly covers legitimate waiting, including slow serial processing of a batch. | Adds to lease duration and therefore the crash-recovery wait. Size against actual queueing/tails, not only the first batch or average handler time. |
| TimeoutGrace | Gives a timed-out, cooperative handler time to unwind before abandonment. | Appears on both sides of the start guard: not extra queue allowance. It cannot stop a Go goroutine that ignores cancellation. |
| RecordMargin | Reserves time for outcome persistence; bounds particular cleanup/record contexts. | Also cancels out of the queue-wait allowance. It is not a global deadline on every database operation in dispatch. |
| ClaimPollRate | Changes empty-poll latency, partial-refill timing and exception tick frequency. | Does not add lease coverage or guarantee shorter waiting; aggressive refill can reserve work earlier. More frequent polling costs more queries. |
| ShutdownTimeout | Controls how long the drain waits for in-flight processing before settlement. | Does not extend leases or fix steady-state queue waiting. Its default follows M, not queue depth. |
| InstanceTTL | Controls the worker-instance ownership heartbeat and replacement timing. | Its heartbeat renews worker_instance, not message claim_lease. It is not the missing message-lease renewal knob. |
| ConfigRefreshInterval | Propagates a redeclared group's timeout/bounds/recovery settings to future claims. | Existing buffered items retain claim-time resolved options. ConsumeOptions are session settings: changing them requires a new Consume session. |
| MaxRangeReclaims; retry limits/backoff; ExceptionInitialBackoff | Control how persistent failures move through range reclaim and individual exception recovery. | They govern recovery cost/pace, not prevention of stale waiting. Faster quarantine can reduce repeated range cycles but introduces individual retries; do not treat successful recovery as healthy first delivery. |
| Concurrency policy and workload separation | Independent work can use parallel execution; separate topics or disjoint bound groups can have different timeouts, queues, and capacity. | Changing ordered/exclusive semantics is a domain decision. Extra differently configured instances of the same group do not route slow versus fast messages automatically. |
| SlowDispatchThreshold; session counters and group snapshots | Show slow handler dispatch, reclaims, remaining leases, backlog, and repeated execution symptoms. | SlowDispatchThreshold excludes time waiting before CallSafely. No dedicated stale-dispatch/queue-age measurement is exposed by that guard. |

For ordinary independent messages, a useful steady-state planning estimate is
`queue wait ≈ ceil(QueueSize / effective concurrency) × dispatch duration`.
The initial batch can wait less. Dispatch duration includes handler execution
and the database work that retains its permit. This is a sizing estimate,
not a hard guarantee: ordered chains, variable runtimes, record retries, and
database stalls must be considered separately. A handler timeout bounds only
the handler call, not every database operation around it.

Workload-specific tuning is established practice:
[Celery recommends shallow prefetch for long-running work and separately configured execution for mixed runtimes](https://docs.celeryq.dev/en/latest/userguide/optimizing.html?highlight=prefetch).
[SQS documents both sizing the lease and extending it during processing](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html).
Vulkan has the sizing controls today; the fresh-message path does not expose
an equivalent per-message heartbeat. Its exception path's conditional renewal
must not be mistaken for general fresh-range renewal.
