# ConsumeOptions batch default: research and local experiments

2026-09-07. Recommendation for discussion: retain `BatchLimit: 1` in the
current implementation. A larger default has a substantial throughput benefit,
but the present lease/queue interaction makes even 4 unsafe as a general
first experience. Resolve that interaction before choosing a larger default;
4 is the first candidate to remeasure, not an accepted new default.

This is a research/review document, not a shipped contract or settled decision.
No library code or defaults were changed.

**What comparable systems actually choose**

| System | Documented behavior | Relevance to Vulkan |
| --- | --- | --- |
| River, PostgreSQL | Claims `MaxWorkers - numJobsActive`; explicitly avoids claiming beyond free execution capacity for distribution and latency. Fetch cooldown defaults to 100ms; fallback polling to 1s, with LISTEN/NOTIFY wakeups. | Strong precedent for tying claimed work to execution capacity. It does not establish a fixed batch of 4, 10, or 100. |
| pg-boss, PostgreSQL | `batchSize: 1`, `localConcurrency: 1`, base polling interval 2s. Optional notification wakeups and continuous fetching modes. | Direct counterexample to “Postgres requires batching by default.” Its batch handler semantics differ from Vulkan's per-message handler. |
| Amazon SQS | Receive defaults to 1, maximum 10. Long polling can return as soon as a message arrives. | Conservative delivery count is established practice; a long-poll timeout is not a sleep after an empty response. |
| Kafka | `max.poll.records: 500`, but this does not control network fetches; fetches are cached separately. `fetch.min.bytes: 1` allows prompt responses. | Large fetches need not wait for a batch to fill. Kafka's partition ownership and fetch cache are different from Vulkan's fixed range leases. |
| RabbitMQ | Guidance calls prefetch 1 most conservative and describes 100–300 as commonly effective for throughput. | Evidence for the cost of tiny prefetch, not a transferable Vulkan default: this bounds outstanding deliveries, not rows sharing Vulkan's lease and commit. |

Sources, accessed 2026-09-07: [River source](https://raw.githubusercontent.com/riverqueue/river/master/producer.go),
[River default constants](https://pkg.go.dev/github.com/riverqueue/river),
[pg-boss workers](https://pgboss.io/api/workers),
[SQS ReceiveMessage](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_ReceiveMessage.html),
[Kafka consumer configuration](https://kafka.apache.org/42/configuration/consumer-configs/),
[RabbitMQ throughput guidance](https://www.rabbitmq.com/docs/confirms).
These are the versions/pages consulted, not claims that every client or release
uses identical defaults. The research supports capacity-aware claiming and
separating wakeup latency from batching; it does not identify a universal number.

**Corrections to the roadmap premise**

`QueueSize: 1` does prefetch: dispatch removes one item while its handler runs,
and the independent prefetch goroutine can fill that vacancy. The default is
one claimed range per message, not zero prefetch. A successful fresh claim
also takes multiple database round trips: snapshot read, BEGIN, cursor query,
lease INSERT, message read, COMMIT. Outcomes have a separate commit transaction.

The fresh cursor advances by an ID interval, using
`claimed = LEAST(c.claimed + $2, gate.head)`. `BatchLimit` therefore limits
fresh ID span, not necessarily returned rows: gaps, schema versions, routing,
and compaction can leave fewer or no matching messages. Reclaim reads the old
range intact; it is not resized to the reclaiming instance's BatchLimit.

Neither `QueueMargin` nor `ShutdownTimeout` grows with BatchLimit today.
The lease is `MessageMax.Timeout + TimeoutGrace + QueueMargin + RecordMargin`:
37.1s with defaults. The derived shutdown drain budget is 32.1s. QueueMargin
remains 5s regardless of queue depth or handler concurrency.

Code: [options](pkg/consumer/consume_options.go),
[queue](pkg/common/concurrency/queue.go),
[runner](pkg/consume/messageconsumer/consumer_runner.go),
[fresh claim SQL](pkg/consume/messageconsumer/controller/datastore/fresh_claim.go),
[reclaim SQL](pkg/consume/messageconsumer/controller/datastore/reclaim.go).

**Measured results**

Environment: repository HEAD `4d875779`, existing working-tree changes present;
library unchanged. Go 1.27.0 darwin/arm64, local container PostgreSQL 17.10
aarch64. Public Vulkan API, normal manager enabled, unique temporary schema
`batch_research_1788815117020399000`, fresh topic/group per run. Schema cleanup
completed successfully; existing installation data was not reset.

Fast test: 2,000 preproduced small unkeyed messages per run; three sequential
trials per batch, concurrency 1, QueueSize following BatchLimit, all other
ConsumeOptions defaulted. No handler I/O or per-message printing. Timing is
first handler entry through the final handler completion, excluding startup
and final shutdown settlement. This is handler throughput, not a measurement
of fully durable completion latency. Final-handler cancellation produced
commit-cancel warnings followed by shutdown settlement; all fast runs returned
nil and observed 2,000 unique messages. No cross-machine or network-RTT sweep.

| Batch / queue | Trial 1 messages/s | Trial 2 | Trial 3 | Median | Relative to 1 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1 | 838.3 | 904.9 | 908.0 | 904.9 | 1.00× |
| 4 | 3455.6 | 3392.1 | 3311.0 | 3392.1 | 3.75× |
| 8 | 5931.5 | 6134.9 | 5986.6 | 5986.6 | 6.62× |
| 16 | 10838.6 | 10153.8 | 9850.6 | 10153.8 | 11.22× |

Slow test: eight unkeyed messages, each handler waits 2s and observes its
context; default 30s handler timeout. One run per batch, separate topics run
concurrently, 46s watchdog. Handler starts were printed to establish sequence.

| Batch / queue | Unique successful messages | Successful handler calls | Result |
| --- | ---: | ---: | --- |
| 1 | 8/8 | 8 | Finished in 16.072s |
| 4 | 6/8 | 9 | Watchdog at 46.071s; earlier successes repeated |
| 8 | 3/8 | 6 | Watchdog at 46.054s; earlier successes repeated |

The slow cases are a mechanism reproduction, not a statistical performance
benchmark. Their timeout is an observed failure to finish, not a passing test.
The experiment executable exited successfully after reporting these expected
watchdog outcomes and deleting its schema.

Reproduction artifacts for this session:
[harness](/tmp/vulkan-batch-research.go), [raw output](/tmp/vulkan-batch-research.log).
Run from the repository with `go run /tmp/vulkan-batch-research.go` against the
example local database. Each run generates and removes its own schema.

**Worked failure: group batch-probe, topic slow-8**

1. Topic ID 16, group ID 17 claims `(0, 8]`, messages 1–8. The claim's
   `expires_at` covers 37.1s: `30s + 100ms + 5s + 2s`.
2. With one handler permit, messages 1, 2, and 3 start at approximately
   0.022s, 2.023s, and 4.023s. Each succeeds after 2s. The range stays open:
   `commitIfResolved` requires every message in it to resolve.
3. Around 6s, message 4 reaches the real guard:
   `ExpiresAt.Before(time.Now().Add(item.options.Timeout + TimeoutGrace + RecordMargin))`.
   Only about 31s remains, less than the required 32.1s. The handler has not
   timed out; the queue has exhausted its allowance. The runner calls
   `markStale(token)` and returns without processing or resolving this item.
   Messages 5–8 fail the same guard.
4. `markStale` sets an in-memory flag; it does not immediately release,
   partially commit, or renew the lease. At 40.045s the same live instance
   reclaims `(0, 8]` after expiry and polling. Messages 1–3 run successfully
   again. The test stops with only three unique messages completed.

The broken premise is that one fixed queue allowance covers all work claimed
ahead of execution. Larger batches amplify an existing issue: even batch 1
can prefetch a message behind a handler lasting more than 5s. That latter case
is a code-derived consequence, not an additional measured trial here.

**Surrounding interactions that constrain the decision**

| Mechanism | Consequence of changing BatchLimit |
| --- | --- |
| Queue refill | With QueueSize equal to BatchLimit, prefetch waits for an entire batch's room, or the 5s timer permits a smaller claim. It need not wait for messages to accumulate in Postgres. Increasing QueueSize can improve overlap while also increasing claimed waiting time. |
| Handler concurrency | Remains 1 unless set. Larger batches amortize claim and outcome transactions; they do not parallelize handlers. For ordinary independent messages, the last of B fresh messages initially waits roughly `floor((B-1)/C) × handler_duration`, before existing queued/in-flight work and DB overhead. |
| Ordered keys | Only a chain's head occupies the queue; remaining same-key messages run serially under the same permit. QueueSize is not a strict bound on all retained message rows. Raising concurrency does not shorten a single ordered chain. |
| Range settlement | Successful handlers are recorded in memory until the whole range resolves. A crash, stale range, or unresolved earlier message expands the repeated-success exposure with batch size. The committed cursor and durable delivery history can lag handler-success counts. |
| Exceptions | The public adapter passes BatchLimit and ClaimPollRate to the exception consumer too. It runs a serial batch on a ticker, with per-message outcome writes and conditional lease renewal before processing. Default 1/5s limits short, continuously ready retries to about 0.2 per second per instance; 4 changes that to about 0.8 absent processing overhead. MessageConcurrency does not govern this loop. |
| Shutdown | Dispatch stops and waits for in-flight calls; ordinary queued messages are not all drained as handlers. Untouched ranges are force-reclaimed; partially resolved ranges retain a suffix. Ordered chains and multiple settlement calls mean ShutdownTimeout bounds the drain stage, not the entire shutdown. Multiplying it by BatchLimit is not automatically the correct remedy. |
| Idle polling | An empty fresh claim sleeps 5s. Increasing batch size does not remove this delay. Uniformly timed arrivals into an otherwise idle, immediately claimable topic incur about 2.5s mean polling delay; this is arithmetic, not a measured latency result. A 1s interval would multiply idle fresh-claim query frequency by five and also accelerate the exception loop. |
| Snapshot fence | Fresh claims wait for a proven head. Transactions holding back the snapshot horizon can delay visibility beyond one polling interval. More batch capacity does not remove this correctness gate. |
| Producer batching | Concurrent produces already share transactions (MaxSize 100, transaction concurrency 4). This can feed bursts, but producer batches do not define consumer ranges or justify reserving slow handler work under a fixed lease. |
| Diagnostics | Reclaim warnings and reclaimed/quarantined counters expose the eventual consequence. The stale-dispatch branch itself emits no warning or dedicated counter. A larger default needs direct queue-wait/stale-dispatch evidence and resolved configuration in startup diagnostics; documentation alone would leave the surprise silent. |

Additional code: [exception runner](pkg/consume/exceptionconsumer/consumer_runner.go),
[public adapters](pkg/consumer/adapter.go),
[range state](pkg/consume/messageconsumer/range_state.go),
[session metrics](pkg/metrics/consumer_session_metrics.go).
Record 0046 settles ownership, not the numeric default. Record 0505 discusses
a proposed breaker threshold; no breaker reader was found in the current
consumer implementation, so it is not counted here as shipped protection.

**Options for discussion**

A. Retain 1 now. Lowest change cost and smallest range replay unit; pays the
measured throughput cost and leaves the existing queue-lease issue unresolved.

B. Change only the default to 4. Material no-op throughput improvement;
reject this option because a healthy 2s handler reproduces stalls and duplicates.

C. Settle claiming versus execution capacity and lease coverage first, then
remeasure 4 as a candidate. River provides a concrete capacity-first precedent.
Any design must cover queued work, same-key chains, reclaimed ranges, exception
renewal, crash recovery, and observable stale dispatch. A larger fixed margin
merely trades this pressure for slower crash recovery; it is not evidence that
the premise is sound. This is design work, not a proposed patch to add helpers
or post-claim caps.

Pick A for today's default and C for the next design discussion. Keep
ClaimPollRate as a separate latency/load decision; neither these throughput
measurements nor competitor defaults establish the right polling cost budget.
No decision record was added because the design has not settled with the user.
