# Roadmap
 0689
Future work, in order of intent. Not a promise — items reorder freely.

- **Now** — committed next work. Picking an item up expands it into TODO.md's
  working window.
- **Next** — agreed, scope still evolving.
- **Later** — intended, roughly sequenced.
- **Parking lot** — no commitment; picked up only if a real workload demands
  it. Ideas move here rather than being deleted.

An item starts as a one-liner and accumulates design notes as sub-bullets.
When a design settles, it gets a decision record in `.docs/decisions/` and the
item slims to a pointer. When work ships, its summary moves to HISTORY.md and
the item is removed.

## Now

- **Search-engine submission** -- after the doc-site sitemap is deployed,
  verify the canonical site property in Google Search Console and Bing
  Webmaster Tools, submit the sitemap in each service (or import the verified
  Google property into Bing), and record the exact operator steps and initial
  indexing result so a future domain move or deployment can repeat them.

## Next

- **PostgreSQL 15–18 compatibility matrix** -- complete further testing on
  PostgreSQL 15 and 16, then publish the supported-version matrix with the
  tested minor versions, verification evidence, and scope of the guarantee.
  - Initial investigation: 102 integration tests across nine packages passed
    with the race detector on 15.19, 16.15, 17.11, and 18.6. These results cover
    a frozen source snapshot, not subsequent retry changes.
  - Verify the settled revision, extending 15/16 coverage through the existing
    signal E2E and applicable compatibility recipes. Keep databases isolated;
    the shared-server integration override requires serialized packages
    (`-p 1`) to avoid schema collisions and transaction-snapshot interference.
  - Establish repeatable verification for 15–18 before publishing the matrix;
    CI currently exercises only 18. Distinguish server-version compatibility
    from PostgreSQL major upgrades and application schema-upgrade guarantees.

- **Chocolatey approval and public-feed validation** [0791] -- sqlstreams
  0.1.2 was submitted successfully; last verified status on 2026-09-12 was
  Submitted/Pending, IsApproved=false. Address any moderator feedback.
  After approval, use a clean Windows runner to install from the public
  feed with `choco install sqlstreams --version=0.1.2 --yes`, verify
  `sqlstreams --version` reports 0.1.2, and verify uninstall. Then add the
  Chocolatey install command to READMEs/docs and record the final proof in
  HISTORY. Packaging, checksum-backed installation from the generated
  nupkg, version output, uninstall, and submission already pass in
  [release run 34724932748](https://github.com/allegedlyreliable/sqlstreams/actions/runs/34724932748).

## Later

- **Distribution follow-ups** -- notarization and Authenticode remain
  deferred until raw release downloads need signing; the Homebrew cask
  currently strips its quarantine attribute. winget and scoop remain
  optional package-manager additions.

- **Attribute the idle fleet's remaining Postgres CPU** [0779] [0781] [0783].
  After the idle-fleet fix, 990 rows on one replica still idle at 0.66
  cores with only 0.25 cores in statement execution, and 9,630 rows
  saturate eight cores with under 2 in statements and never finish
  registering. The observer sees statement execution only; teach it plan
  time (`pg_stat_statements.track_planning`), pg_stat_statements'
  deallocations (42k+ entries at 1,600 streams, 100 ms per lab read), and
  background process CPU before choosing a rung for 10k rows. The
  largest remaining statement term is the consumers' 500 ms claim poll,
  not the fleet's upkeep. The stream janitor's sweeps return no counts,
  so it can log nothing either.
- **Automatic-batching capacity comparison** — measure the ordinary automatic
  batching path before making a capacity claim about it. The published explicit
  batch workload is not a substitute; the retired 30-second probe was not a
  capacity measurement.
- **Recovery under load** — choose a fault and recovery objective before adding
  a scenario. Measure recovery time, backlog drain, and replay with durable
  identity evidence; graceful instance-count changes do not simulate crashes.
  - Chaos-run shape drafted 2026-09 (moved off the former benchmark page
    2026-09-11, tooling is not proposed on the site): one hour on `orders`
    with DeliveryLogMode all and a 0.02 handler fail rate; producer phases
    warm steady 200/s 10m, pause 1m, saturate 64 in flight 5m, ramp
    2000/s -> 200/s 10m, hold 200/s; consumers 3 at 0m, 0 at 20m, 8 at 22m,
    kill consumer 2 at 45m. Expect lost/unexpected/undelivered/unbucketed 0,
    recovered and duplicates reported, reclaims 0 outside the kill window,
    recovery <= 2m after the pause, the flood, and the kill. Two phases are
    deliberate: consumers at zero forces backlog growth and drain with
    nothing lost; eight on a three-consumer stream is oversubscription with
    zero reclaims still expected. A kill on an idle instance, or a run
    whose lease never expires, reads unknown. Staged after that: Postgres
    paused past the lease, a compacted keyed stream with a "compacted away"
    bucket and per-key order, two schema versions live, bindings and
    schedules, `__system` metric and alert assertions at the end.
- **`sqlstreams demo --chaos`** — one command that starts a disposable
  Postgres, an `orders` stream, three consumer instances, and 5,000
  messages, prints produced / resolved / lost, and suggests attacks
  (kill an instance mid-message, crash mid-commit, restart Postgres) so a
  reader verifies durability by hand in five minutes. The proposed page
  was removed 2026-09-11: its premise (the packaged failure-injection e2e
  tests) went when those tests became integration tests, and no scenario
  driver exists yet. Pickup depends on the benchmark manager owning
  complete runs; the demo is that manager's run with a scoreboard.

Pre-v1 — the 14b public-API pass, then measurement, evaluation, and
documentation; the latter want a surface that has stopped moving.

- **Whole-partition retention with explicit maximum-timestamp metadata.**
  - Reassess after [0734]: user accepts approximate id/timestamp order;
    evaluate the oldest-row sweep precheck before this larger redesign.
  Replace repeated per-row expiry scans with partition lifecycle management
  that drops sealed partitions only when their greatest message timestamp
  has expired and consumer cursors permit deletion. User selected this as
  the next design direction after native retention benchmarking exposed
  million-row scans returning no expired rows and janitor timeouts.
  - Preserve protection for lagging consumers and transactional cleanup of
    associated rows. Highest message id does not imply newest created_at:
    concurrent transactions and NOW() transaction timestamps can invert
    their order. Resolve that correctness risk explicitly; timeout tuning
    does not fix it.
  - Define partition sealing/rotation for low-volume and idle streams, safe
    maximum-timestamp maintenance under concurrent/late commits, and the
    extra retention beyond TTL caused by whole-partition deletion. Surface
    retained bytes, expiry backlog, oldest retained age and cleanup failures
    so operators can see delayed cleanup rather than silently growing disk.
  - Benchmark metadata write contention, WAL/index cost and steady paired
    throughput over several cleanup cycles within the storage budget.
    Idempotency-key expiry remains a separate cleanup workload.
  - Interim: test longer janitor cleanup deadlines and polling intervals in
    scratch benchmarks; keep current row-level semantics. Earlier scratch
    evidence was retired; collect fresh evidence when this work resumes.

- **Consider successful maintenance-pass tracking** -- evaluate whether janitor
  and vacuum status should expose the last completed pass separately from
  claim liveness. Decide whether that visibility justifies extra database
  writes, an index, and retained history; define retention and failure behavior
  before choosing an implementation.

- **Consider staggering the first maintenance pass** -- measure whether many
  streams starting vacuum together cause a meaningful database load spike.
  Compare immediate execution with a randomized initial delay, including the
  cost of postponing cleanup. Add scheduling configuration only if warranted.

- **Metric or alert for lagging dead-tuple reclamation** -- identify streams
  whose deleted idempotency-key rows accumulate faster than vacuum reclaims
  their space. Evaluate trends in estimated dead tuples, retained table/index
  size, and vacuum progress/completions; distinguish expired rows awaiting
  janitor deletion from deleted rows awaiting reclamation. Use a sustained
  condition rather than one high count, and avoid attributing every backlog
  to autovacuum without supporting evidence. The operator guidance should
  explain when to enable the stream's scheduled vacuum worker and how to
  verify that reclamation catches up. Reuse the existing metrics/alert
  machinery; define sampling cost and alert thresholds when this is picked up.

- **Benchmark scenarios declare the janitor's TTLs** -- the quiet run
  registers `orders` with defaults, so every janitor sweep returns before
  touching a row: retention 0 disables drop and sweep, the 24h idempotency
  key TTL never expires inside 60m, and 720k rows never fill a 1M-row
  partition. A green run proves 720 heartbeats and nothing about the
  question the janitor poses -- does a drop or sweep ever take a row a
  lagging group has not been delivered. `--time-scale` scales phases, not
  TTLs, so it cannot help. Fix: the scenario's `[input]` stream line carries
  `RetentionTTL`, `PartitionSize`, and `IdempotencyKeyTTL`, `Scaled`
  scales the two durations with the phases, and the quiet run declares
  `RetentionTTL 5m PartitionSize 10000 IdempotencyKeyTTL 5m` -- 72
  partitions filled, most dropped, keys swept from minute 5, with
  `AllowDropPastCommitted` left false so the existing `lost 0` and
  `undelivered 0` become the janitor's checks. Validation rule at parse
  time: every TTL is at most duration/5 and the partition size at most
  produced rows/5, so each path fires many times per run. Open cost:
  dropped partitions take their delivery_log rows, so the `duplicates`,
  `dead`, and `reclaims` checks that read the database lose evidence for
  swept messages -- move them to the handler ledger files, which hold
  every delivery, before turning retention on.

- **Claim stall Warn** -- a produce inside a caller-owned transaction
  holds every consumer group on the stream at that message id until the
  commit, and today the only symptom is lag. Add a declared Warn event on
  the consumer side when a claim has waited on an uncommitted message
  longer than a threshold, carrying stream, group, the message id it is
  held at, and the stall duration. Surfaced by playground scenario 04;
  the guide (transactional-produce) states the rule in prose, this is the
  observability half.

- **Dead-lettered messages: list + retry on the consumer handle** -- a
  dead row sits in exception_queue_<stream_id> with status dead,
  last_error, and attempts, and the client has no verb to read it or put
  it back; today the read is the SQL0028 diagnose query in psql and the
  write is a hand-written UPDATE. Add `Consumer(...).Exceptions(ctx,
  status, limit)` returning the rows and `Retry(ctx, messageId)` setting
  dead -> ready, with CLI `group exceptions list|retry` and a docs page.
  A list with no action on the same surface is half a feature, so the
  pair ships together. Surfaced by playground scenario 03.

- **Compacted key Update verb + missed-opt-in Warn** -- read-modify-write
  on a compacted key is an unnamed three-step pattern (InTransaction +
  LockCompactionHead + ProduceInTx with MessageKey and Compaction repeated
  on the produce), and a keyed produce that forgets the Compaction option
  is silently never a version of its key. Add
  `Key(k).Update(ctx, func(current *Message) (*Message, error))` on the
  key handle wrapping the three steps and setting MessageKey + Compaction
  itself (the JetStream KV shape: Get returns the revision, Update is the
  conditional write), plus a declared Warn when a keyed, uncompacted
  message lands on a stream whose compaction_head already holds that key.
  CONCERN: the Warn needs a compaction_head lookup on the produce path,
  which is hot -- extra latency per keyed produce is the cost, so it
  ships only if the lookup rides a statement produce already runs, never
  as its own round trip; otherwise drop the Warn and keep the verb.
  Surfaced by playground scenario 13.

- **Rewind an existing group + Start-ignored Warn** -- `ConsumerConfig.Start`
  is read once, when Register creates the cursor row; on an existing group
  a changed Start changes nothing and logs nothing, since Start is not part
  of the stored config. Two halves: (1) a declared Warn at Register when
  the supplied Start names a position other than where the existing cursor
  sits -- once per Register, off the hot path, carrying stream, group, the
  requested position, and the committed id; (2) the rewind verb the replay
  guide (website guides/replay, marked Proposed) already specs, with
  `AtMessageId` / `AtTime` positions, which is the only way to move an
  existing group. The guide is the spec; this line is its owner. Surfaced
  by playground scenario 07.

- **Doc-site breadcrumb structured data** -- emit `BreadcrumbList` JSON-LD
  from the same trail each page already renders, so the machine-readable and
  visible hierarchies cannot disagree. Validate representative board, guide,
  code, and decision-record pages with Google's Rich Results Test after
  deployment; this improves result presentation but is not an indexing gate.

- **`diagnostic.MetricScope` -> `diagnostic.Scope`** — alerts share the
  metric scope type since [0649], so its name is stale. The rename touches
  the diagnostic package, both definition views, the explain document, the
  code export, and the site's record types; deferred from the alerts
  close-out because that diff is wider than the work it rides on.
- **Proposal pages for public discussion** — give substantial ideas a specific
  page that states the proposal and links a discussion where readers can leave
  thoughts, rather than making an issue or a finished documentation page carry
  both roles. Pickup must settle the home for the discussion, identity and
  moderation, and how a proposal progresses into shipped documentation.

- **Publish selected roadmap items as GitHub issues** — near public launch,
  use an agent to turn externally useful Later items into reviewed GitHub
  issues, inviting reactions and discussion. `.docs/ROADMAP.md` remains the
  ordered source of truth; issues are public discussion and interest signals,
  not commitments. Settle selection criteria, labels, issue status when work
  ships or is dropped, and how issue discussion feeds back into the roadmap.
  - The contributor workflow settled under Contributor documentation (Next)
    answers most of this: an item gets an issue when it has a decision record
    or sits in Now (`accepted`, `help wanted`), otherwise `roadmap`; the
    issue closes with its HISTORY.md entry linked; discussion feeds back as
    sub-bullets on the item.

- **Worked contribution examples** — after the contributor workflow has run
  on real changes, pick two or three shipped ones (one bug fix, one public
  API change that went issue -> Proposed page -> decision record ->
  implementation) and write each up as a follow-along: the issue as filed,
  the triage reply, the Proposed page diff, the record, the PRs, in order,
  with what a reviewer pushed back on. Lives on the doc site beside the
  contributing guide, not in .docs/. Depends on the issues item above having
  produced at least one contributed change to point at.

- **BindingHandle verbs beyond Get** — the handle shipped [0645]; these
  are bare nouns like every handle verb: `Waiting(ctx)` the
  group's still-blocked declarers (same datastore read, the
  controller's `openWaiters`); `Log(ctx, limit)` set-change history
  newest first (a new query
  without DISTINCT ON -- the first `_config_log` listing verb, so it
  sets the shape stream_config_log / worker_config_log would copy);
  `Matches(ctx, routingKey)` an `EXISTS ... ~ pattern_regex` read on
  binding_config, the claim query's own predicate -- client-side
  matching would be a second mechanism for the same fact. Per-stream
  `StreamHandle.Bindings(ctx)` beside `Consumers` if a caller
  wants it (the datastore already reads per stream). Never `Declare` /
  `Clear`: [0511] removed them -- with live instances an admin declare
  waits forever, with none the app's next Register overwrites it.
- **Metrics handles for consumer, producer, and possibly scheduler** — after
  the System / Stream / Consumer metrics surface settles, evaluate metrics on the
  running `ConsumerInstance`, `ProducerInstance`, and `SchedulerInstance`.
  These must expose facts owned by that process or session, not duplicate the
  Consumer / Stream snapshots or give a second path to stored metric history.
  Consumer session counters already provide a candidate; producer metrics need
  a settled lifecycle, and scheduler earns a handle only if it has meaningful
  instance-local facts rather than fleet state that belongs to System. Specify
  identity, lifetime, freshness, and what remains readable after the instance
  stops before proposing methods.
- **A declaration reports what it did** — `Declaration`
  (created / joined / updated) on the consumer and schedule instances,
  carried as the "Reading the outcome back" aside in concepts/api-shape.mdx.
  Cut from the [0625] chunk 12 build, and the strict form it was built
  beside is rejected outright ([0626], parking lot), so this is now the
  whole of what chunk 12 might still be worth. SQL0059 already reports an
  overwrite as a Warn and the value bought is reporting only — an API
  for what the log says. One thing to settle before it is worth
  building: where a schedule's outcome would live, since
  `client.Scheduler(name).Register` returns an instance, so the outcome can
  live on that product of one call. The option is returning
  `*SchedulerInstance` with the client's `SystemManager`
  passed in (a `Schedule` call builds a manager per call, which
  scheduleconcurrency covers -- rival loops are no longer the
  objection, since the row's claim gate arbitrates them) or dropping
  the schedule half. The plumbing
  was built once and reverted in full — a declaration verb returning the
  outcome and the instance carrying it — so the shape is known.

- **Diagnose queries filled from the values a raise attached** — today a
  declared query renders with its `{stream_id}`/`{schema}` placeholders
  literal, in `sqlstreams explain <code>` and on the error pages, and the
  reader substitutes. `Error.Fill` already exists and runs on the fix;
  pointing it at `Query.Sql` and rendering the result in
  `renderErrorBlock` would hand an operator a query they can paste
  as-is. That is what makes attaching values at every raise site worth
  the plumbing — the [0625] chunk 15 task 5 settlement assumed the
  filling existed, and it does not.

- **"Schema version" for a migration version** — deferred 2026-09-01
  during [0629], which gave `schema` one meaning everywhere else.
  SQL0022/SQL0023's problem lines, `ErrSchemaOlderThanBuild` /
  `ErrSchemaNewerThanBuild`, and roughly 8 files of prose still say
  "schema version" about a migration version. The rule if it is taken
  up: `migration_log.version` reads "migration version" and
  `message_log.schema_version` keeps "schema version", so
  guides/schema-versions.mdx, which is about payloads, never moves.

- **`schedule_config` declaration trail** — surfaced by the 2026-08-30
  init-model rethink (guides/consumer-group-config.mdx): stream, worker,
  and binding declarations all keep a `_config_log` trail;
  schedule_config has none, so a redeclared cron expression leaves no
  history. Decide whether it earns a `schedule_config_log` on the same
  full-snapshot pattern.

- **Voice workflow rungs 2–3** ([0609] shipped rung 1, the
  .website/VOICE.md file; these are deferred until the author has
  time). Rung 2: author-seeded drafts — the author types or dictates
  the rough take first and the AI continues and tightens, plus one
  critique pass naming the draft's differences from the samples
  before revising; judging is comparative only ("which passage is by
  the samples' author"), in a fresh context, never a score. Rung 3:
  after a handful of pages accumulate AI-draft → published-edit git
  diffs, periodically distill what the author changed into VOICE.md
  amendments (replacing the constructed contrastive pairs with real
  pairs) and keep a dismissed-patterns note so rejected ideas are
  not re-proposed. The research evidence is summarized in [0609];
  VOICE.md ## Sample sources names where future samples may come
  from.

- **Upcaster for skipped schema versions** (after [0618] ships): a
  consumer group today skips rows whose `schema_version` differs from
  its Message type's. An optional per-consumer decoder --
  `ConsumerConfig.Upgrade` shaped roughly `map[int]
  func(json.RawMessage) (*Message, error)` -- would let one group read
  older rows through a user-written converter instead of a bridge
  re-produce. Plugs into the claim predicate (versions with an
  upgrader become claimable) and the decode step; Confluent's reader
  schema / Temporal's data converter are the precedents. Not before
  the skip behavior has been lived with.
  - The key reads have the same gap from the other side [0646]:
    `Stream[OrderV1].Key(k).CompactionHead` on a key whose head is V2
    decodes the V2 payload into the V1 struct silently. "not found"
    is the wrong word for it; the answer is whatever the upcaster
    decides, so it waits here with it.

- **Doc-site pages the 2026-08-28 link sweep found missing** — a
  compaction concept page (concepts/message-key now covers the basics
  [0612], but the deep mechanics — compaction_head, rank rules,
  retention interplay — still have no page) and a workers/maintenance-fleet
  page (the fleet is a table in concepts/architecture plus one
  quickstart caution; `sqlstreams manager run` and schedules have no home).

- **`sqlstreams explain --run`** (or a `sqlstreams diagnose` verb) — execute a
  declaration's diagnose queries against the operator's own database, since
  the CLI already holds a connection. The queries themselves shipped
  2026-08-25 [0589]; placeholders named by attribute key keep this reachable
  without a redesign, and the CLI would take `--stream-id`-style flags.

- **Vocabulary walker** — enforce the CONVENTIONS.md ## Vocabulary registry
  mechanically: a .tools/conventions test that greps code, comments, and
  .website/ prose for banned terms (allowlisting the registry itself and
  historical docs), so the table is enforced, not advisory.

- **Marketing** I know there are other kafka in sql projects out there
  why did they fail? Was it product or marketing related? what can we learn
  from those failures?

- **Circuit breaker implementation** (Phase 16, post-v1; two-tier design
  settled in Phase 13 — per-instance trip unit, quorum globalization,
  refund-on-close reconciliation; only questions explicitly left for
  implementation time reopen here):
  - error_class enum on delivery rows, recorded at exception time from the
    user's classification; values coordinate with 14b's named-errors
    taxonomy. The one schema touch, lands first.
  - Per-instance breaker: local streak tracking (N non-empty all-systemic
    ticks + M cumulative, exception retries counting), open state gating
    claims/retries/buffered work; state read from an atomic refreshed by its
    own async ticker, never a hot-path query.
  - Shared breaker row + globalization: (stream_id, group) row, guarded
    CLOSED->OPEN with a generation counter; settle quorum K here (small
    absolute vs presence-backed fraction — if fraction wins, presence
    heartbeat rows become a prerequisite; see parking lot).
  - Probe paths: local self-probe on cooldown; global prober elected via
    session-level advisory lock (self-releasing on crash); half-open exists
    only as OPEN + holding the lock; global close does not force local
    closes.
  - Reconciliation on close: probe winner refunds attempts AND reclaims
    systemic-classed rows (dead -> ready); the hand-back question for
    claimed-but-unattempted work resolves here (PartialCommit + refunded
    reclaim vs a new explicit release).
  - Config + observability: sparse opt-in MessageConsumerConfig fields (trip
    threshold N/M, cooldown, probe size, quorum); per-instance and group
    state metered + queryable; "instance open, group closed" surfaced as the
    bad-node signal; DLQ alerting able to report "breaker open, N dead rows
    pending reconciliation".
  - Breaker e2e test: dead-dependency (all trip -> global OPEN -> recovery ->
    zero wrongful DLQ), bad-node (one instance trips alone, group drains,
    its rows succeed elsewhere), flap (cooldown backoff + refund cycles
    converge, no poison-quarantine creep).
  - Real systems: Envoy outlier detection (tier 1), Resilience4j/Polly
    (classic in-process), Finagle failure accrual; the two-tier composition
    is per-host ejection + cluster-wide panic thresholds.

## Parking lot

Post-v1, unordered. Pick up only if a real workload demands it. Known
dependencies: pgx-vs-database/sql should weigh LISTEN/NOTIFY's outcome if
both are in play; presence heartbeat rows are the circuit breaker's
prerequisite if quorum-as-a-fraction wins.

- **Repeatable release assurance** — tie publication to successful verification
  of the tagged revision, including applicable compatibility checks, and retain
  installation results for supported distribution paths. Start with the existing
  verification, signal, and compatibility recipes and recorded release evidence.
  At pickup, settle how publication requires that evidence and which installation
  checks run for each release; add automation only where the existing workflow
  leaves a concrete gap.

- **Two idempotency claim horizons** -- every produce writes an
  idempotency_key row, minted key or not, so `IdempotencyKeyTTL` is the
  claim table's size; the minted-key path only needs the claim to outlive
  one call's retry curve (minutes) while a caller key must outlive the
  upstream's retry horizon (a day). Split them: a `caller_supplied`
  column on the claim row (a caller's UUID string is stored verbatim and
  cannot be told from a minted v7), minted claims swept after minutes,
  caller claims kept for the stream's TTL, two sweep predicates in the
  stream janitor. Pick up only if a real caller-key workload shows the
  24h default (restored 2026-09-05 to [0283]'s value from an unrecorded
  1h) costing measurable WAL or sweep time past the 10M-row bench floor.

- **Fresh claim as one pipelined batch** (parked 2026-09-06) -- collapse
  the fresh-claim transaction from five round trips (BEGIN, cursorSql,
  lease INSERT, readMessages, COMMIT) to one. Measured 2026-09-06 on a
  local docker Postgres (RTT ~65µs, 41 partitions, BatchLimit 100): the
  claim's SQL executes in ~150µs server-side across all statements; the
  rest of a 1120µs claim is round trips, one WAL fsync, and payload
  transfer. A prototype of this shape measured 788µs per claim and
  981 -> 1345 claims/s with 8 instances on one group. On a 1ms network
  the same shape is ~6ms -> ~2ms. Pick up only when a networked
  benchmark shows claim latency or the cursor-row lock as the limiter;
  the idle poll is already one round trip.
  - The shape: the caller mints the lease token (the key lease already
    does this), the cursor UPDATE and the lease INSERT fuse into one
    statement, and the read finds its bounds through that token. The
    two statements go out as one `pool.SendBatch`, which is one implicit
    transaction (pgx's documented contract; partition.go's SET LOCAL +
    advisory xact lock batch already depends on it). The cursor-row lock
    is then held for server execution only, not across client round
    trips -- that is the fleet-wide claim ceiling for one group.

    ```go
    token := uuid.NewV7()
    batch := &pgx.Batch{}
    batch.Queue(claimSql, groupId, limit, snapshot.Head, snapshot.Xid, leaseSeconds, token)
    batch.Queue(readSql, token, groupId, schemaVersion)
    results := pool.SendBatch(ctx, batch)      // one round trip, one implicit transaction
    low, high, leased := results.QueryRow()    // claimSql; zero rows = no cursor row
    messages := results.Query()                // readSql; empty when nothing was leased
    results.Close()                            // implicit COMMIT
    ```

    `claimSql` is today's cursorSql plus one CTE; the caught-up case
    inserts nothing, which replaces the Go `low >= high` guard:

    ```sql
    lease AS (
        INSERT INTO claim_lease (consumer_group_id, token, low, high, expires_at)
        SELECT $1, $6, u.low, u.high, now() + make_interval(secs => $5)
        FROM updated u
        WHERE u.low < u.high
        RETURNING token
    )
    SELECT u.low, u.high, EXISTS (SELECT 1 FROM lease) AS leased FROM updated u;
    ```

    `readSql` is today's readMessages with its bounds read from the lease
    row. Scalar subqueries, never a join -- a join plans the bound as a
    filter over the whole index ([0391]):

    ```sql
    WHERE m.id > (SELECT low  FROM claim_lease WHERE consumer_group_id = $2 AND token = $1)
      AND m.id <= (SELECT high FROM claim_lease WHERE consumer_group_id = $2 AND token = $1)
    ```
  - Extent: `freshClaimMessagesWithCursor` and `claimMessages` merge into
    one method (~60 lines net); `ConsumerGroupCursorRow` gains `Leased`;
    the reclaim path keeps the (low, high) read, so readMessages either
    stays as a second literal or the reclaim also reads by token. No
    schema change. The missing-cursor error still comes from zero rows on
    the first result ([0387]).
  - E2E test shape to add: the caught-up branch inside the batch (no lease
    row, empty read), and a peer's claim blocked on the cursor row still
    reading the bounds its own lease carries.
  - Rejected on the way: bounding MAX(id) by settled_head for partition
    pruning (turns the InitPlan into a correlated SubPlan, measured
    slower; the probe already stops at the first partition with a row),
    and reading the sequence's last_value as head (the sequence read
    happens after the statement's snapshot, so an id issued in between by
    a transaction with xid >= xmax escapes the fence).

- **Custom user metric definitions** — let applications declare the name,
  kind, unit, description, and attribute keys of their own metrics so
  `System().Metrics().Definitions()` can discover them before the first
  measurement. The first-class metrics work deliberately lists SQLStreams
  built-ins only; user measurements remain self-describing and discoverable
  through `Latest(ctx)` in the meantime. Do not infer a definition from an
  observed measurement: pickup must settle durable registration, ownership,
  reserved-name enforcement, and what happens when a redeclaration changes
  kind, unit, or attribute keys. Promote only when a user metric needs
  pre-measurement discovery or shared help text rather than merely history and
  export.
- **Strict declaration forms: `RequireMatch` and the stale-build gate**
  (parked 2026-08-31, cut from the [0625] chunks 12 and 13). Both are a
  lock with no key. `RequireMatch` (chunk 12, built and reverted in
  full): a service sets it and the group's config can never change
  again without redeploying that service with it false, deploying the
  change, then setting it back -- nobody wants that dance. The
  stale-build gate (chunk 13, never built): refusing a declaration from
  a build whose document would drop a newer build's fields turns every
  rollback into an outage -- the old build's `RegisterConsumer` is
  refused and no verb lowers the stored floor. It also has no premise on
  the migration gate's own terms: a new config field is additive (older
  builds run without it, absent resolves to the default), which is
  `MinCompatibleVersion = 0`, the floor that never locks anyone out; and
  the sparse `omitempty` document cannot tell "old build" from "unset"
  without a version stamp plus a hand-kept field->version registry. The
  dropped field is source code -- it comes back the moment the newer
  build declares again. What stands instead: newest-wins with the
  differing-overwrite warn (SQL0059) naming the change. Re-examine only
  with a concrete workload where the warn was not enough, and any strict
  form must ship with the verb that unlocks it.

- **File the view-transition `ready` leak upstream on Astro** (parked
  2026-08-27 [0604]). Their router attaches no handler to the promise,
  so every Astro site on mobile Chrome banners a skipped cross-fade as
  a failure; Nuxt fixed the identical leak in its own router (PRs
  #34515, #35537) and their closed report is #10830. Our
  `astro:before-swap` catch is deletable the day they take it.

- **Doc-site sandbox extensions** (parked 2026-08-25 — the sandbox works
  as shipped; each of these is a second story on top of it, none of them
  blocking):
  - Fail-the-next-message toggle on a sandbox consumer, so a delivery row
    materializes at ready -> inflight -> dead while the cursor moves past
    it. The sharpest demonstration the site can make of "success writes no
    row". The three statements it needs (deliveryStatement, logStatement,
    partialCommit) are the ones [0586] left unextracted.
  - Declare a binding on a group, so routing_key selects instead of
    decorating. A bound group claims the full range and reads only the
    messages whose routing_key matches its pattern — the fan-out story,
    and the only thing that earns routing_key a column back in the
    message_log panel (dropped from the default query for exactly that
    reason). Needs UI to declare the pattern, and reintroduces ranges that
    read `· 0 messages`, so it wants page copy alongside it.
  - Try-it links into the sandbox — a link sets a panel's SQL and runs it,
    letting doc pages deep-link example queries. (Wording predates the
    sandbox: ConsoleState is gone; the panels own PanelState over one
    shared DatabaseState.)
  - Inline "why?" toggles expanding the decision record behind a claim
    (liked, unscoped).

- **Doc-site mechanisms considered and not taken** (2026-08-23 brainstorm;
  revive only if the site needs them): a SQLStreams-powered real forum behind
  the board skin (deferred as premature — the board is a static skin
  [0583]); tier 2 of the SQL console, running Go wasm against PGlite
  through a pgconn DialFunc bridge on one flagship page ([0584]); the
  quickstart as a verifiable psql transcript; a retry-curve slider
  playground (judged not unique). Rejected outright: a your-deployment
  context panel and a schema atlas (an interactive column-level map of the
  schema — scrapped 2026-08-24). The log-line-to-investigation-kit idea
  was revived the same day in its declared form and SHIPPED as the
  declared queries [0589] plus the paste box that fills them [0590].
  Prefetching the sandbox's PGlite wasm was built and reverted the same
  way 2026-08-26 — the code-to-gain ratio lost, not a defect [0591];
  reopening it needs a browser measurement, not another estimate. The
  initial-JS byte ceiling went the same way 2026-08-26 [0594]: ~240 lines
  of hand-written import-graph walk plus tests and rule edits behind one
  number that moves a few times a year, so Playwright stays an unused
  stack row and no ceiling is enforced. The measurement stands — the
  homepage is 34.76 KB gzipped / 88.15 KB raw once inline scripts are
  counted, not the 32.30 / 82.17 recorded before — and reviving it wants
  an off-the-shelf checker that is one config file, not a walk of our
  own.

- **Worker-instance stop-line counters** ([0567] follow-on) — the
  standalone worker instances (janitor, schedule producer, metrics
  collector, cursor advancer) still log identity-only stopped lines;
  each would keep its own local lifetime totals (swept, jobs produced,
  measurements collected, advances) and render them the same way. No
  threading needed — each instance is its own tick loop. The CONVENTIONS
  wording ("every lifetime counter the instance keeps") already covers
  counter-less lines until then.
- **Log-viewing as product** (post-v1 rungs from the logging research,
  [0558]): a `sqlstreams tail`-style verb with --stream/--consumer/--level filters
  (Laravel Pail / heroku logs -t precedent); a per-delivery "full story"
  CLI view assembled from delivery_log + deliveries (Telescope/Rails
  request block as CLI); an OBS-loganalyzer-style script diagnosing common
  misconfigurations from any pasted log — feasible exactly because [0558]
  fixed the key registry and static messages; a piped-log annotate mode
  joining SQL codes to their declarations (journalctl -x shape) extending
  `sqlstreams explain`.
- **Debug-buffer extensions** ([0559]) — AutoFlushDuration (.NET log
  buffering: after a drain, forward live for N seconds — the aftermath is
  usually the interesting part) and the Warn-as-drain-trigger revisit
  already recorded in [0559]; pick up when a real incident wants them.
- **Standing-state re-emit** — the start-line snapshot rotates away in a
  months-running process, breaking "any pasted log answers what was your
  setup" (FoundationDB re-logs standing state on every file roll). A
  low-frequency re-emit or an on-demand CLI verb.
- **Hardcoded-config audit** — sweep the library for internal constants a
  user might reasonably need to tune and decide, per constant, whether it
  becomes a Config field (WithDefaults keeps today's value) or stays fixed
  with the rationale recorded. Known candidates: logging's
  logBufferMaxRecords (64) and suppressionWindow (1 minute); the schedule
  snapshot's flat 10-minute overdue threshold; the consumer group
  janitor's waitingDeclarationTTL (7d, [0573]); expect more.
- **Mechanical enforcement of checkable conventions** — a `just vet`
  analyzer (or e2e test) that fails on the CONVENTIONS.md rules a
  machine can check: `SELECT *` anywhere incl. CTEs, banned words in error
  problem lines, tense-follows-recovery, receiver-letter rule, `db:` tags
  on scan structs, Wrap-pair shape, config file naming. Rationale: prose
  rules are followed probabilistically by agents (~70-80% ceiling,
  rule-count decay); every rule the vet layer owns stops taxing adherence
  to the rest. Revisit splitting/scoping CONVENTIONS.md only if violations
  persist after this ships ([0550] context).
  - Error docs-page drift check (user-settled 2026-08-19: pages are
    hand-written, NEVER generated): a CI script that walks the registry
    (.tools/conventions) and fails when a code has no page under
    .website/src/content/docs/errors/ or the page title no longer matches
    the declaration's verbatim problem text; plus an agent-facing hook so
    an agent editing a declaration is pointed at the stale page in the
    same change.
- **FIFO partitions** (Phase 12) — ordering on demand, paid only where opted
  in. `partition_key` on message rows (nullable = no ordering; a second key
  beside message_key on purpose: message_key is a read-time "what's
  current" filter, partition_key a claim-time "don't run two at once" gate
  — sketch predates [0612], which already made message_key defer's
  claim-time gate; re-derive on pickup).
  The bare claim-from-log path is unordered under concurrent workers, so
  FIFO is an opt-in on the lifecycle path: keyed claim skips rows whose key
  already has an in-flight delivery in the group (null key = full
  concurrency); a single cursor reader in id order is the trivial K=1 case.
  Keyed lanes at the dispatch point: same partition_key -> same lane, each
  lane sequential — claimBuffer.WaitForNext (pkg/consumer/claimbuffer.go) is
  the single dequeue point every dispatched message passes through, so a
  lane-routing policy slots in without touching prefetch/Add/resolve. Order
  through a retry is the subtlety: only the lowest unresolved offset of a
  key is eligible, so a backed-off head blocks its later offsets (and a dead
  head stops blocking). Also the principled fix for the Phase 3.5 claim
  hotspot — sharding the claim by key spreads workers across index ranges.
  Hot key serializes to single-worker throughput by design. Real systems:
  Kafka partitions, Pulsar Key_Shared, SQS FIFO MessageGroupId.
- **User-initiated defer & policy-driven dispatch** (12b) — consumers control
  neither WHEN a message comes back nor WHICH pending message runs next.
  - The feature: consumerFunc says "can't process NOW, retry at T" without it
    counting as a failure (downstream rate limit, keyed dependency outage —
    the circuit breaker hands the dead-tenant case here — out-of-order
    business state, off-peak scheduling). Open shapes: a named error
    variable the library recognizes (least churn, composes with the
    named-errors taxonomy) vs richer return; substrate = exception
    window's can_run_after (exists today) vs the ordered-index buffer;
    does a defer consume an attempt, does it write a delivery_log row. A
    defer-only need does not justify building the orderer.
  - The mechanism sketch (async ordered-index claim table, for whenever the
    lifecycle path revives): deliveries stays the durable unordered backlog;
    an orderer async top-ups a SMALL ordered ready-buffer per user policy
    (priority, delay, load shedding); claims pop the buffer head. Ordering is
    WINDOW-APPROXIMATE, not global (Sidekiq/Celery precedent — document it);
    deep-backlog strict priority is restored with low-cardinality priority
    TIERS (one id-ordered orderer cursor per tier, merged by weight); the
    buffer stays only slightly ahead of claims (bounded depth; it's DERIVED
    state — resume story is truncate-and-re-score); the orderer is another
    fenced scanner with a mark, same machinery as fanOut. Open: separate
    buffer table vs nullable position column (position updates lose HOT);
    where policy inputs live (delivery-row columns vs the headers JSONB that
    header routing would add).
  - Could redo both concurrency-deferral and exception claiming on this
    table: retries land in the unordered backlog, materialize near the front
    soon after; a concurrent defer waits to enter the ordered index until
    the message key frees.
  - Overload policies to mine from the Uber resilient-DB talk
    (https://www.youtube.com/watch?v=g7FmEc5GLWs&t=387s): FIFO->LIFO as a
    load-shedding gauge (lag growing -> skip older work until caught up);
    priority tiers 0-5 doing double duty for shedding; producer-side
    backpressure at enqueue is the unexplored half.
  - A revived lifecycle path must also wire the exclusive-consumption key
    gate (consumerBase.claimKeyedRun) — DeliveryConsumer predates it and
    would run keyed Defer messages ungated.
  - Real systems: SQS DelaySeconds/ChangeMessageVisibility; Pulsar
    reconsumeLater + delayed-delivery tracker (exactly the derived
    ready-buffer shape); Sidekiq weighted queues / Celery priorities.
- **Shard the hot lane** (6.5d) — K lanes per group, each owning a frozen
  contiguous block of the log, draining independently; only if a single
  group's frontier is provably contended. Frozen + contiguous means no
  overlap and no seam: lane s owns (H*s/K, H*(s+1)/K], claims cap at the
  lane's block_hi. The exception term in Advance must be lane-scoped or one
  lane's stuck exception freezes every lane. The group's contiguous
  watermark is the committed of the FIRST lane not yet at its block_hi, not
  min(committed) (which sticks once lane 0 finishes). Striping by
  offset % K is wrong — a dense single-integer cursor can't represent it.
- **Header/content routing** (7b) — routing_key matching is
  positional/hierarchical; some routing is about an unordered attribute set
  ("region=eu AND tier=gold"). Add `headers jsonb not null default '{}'`;
  binding grows a discriminator (kind column) once there are two matcher
  shapes; header matcher is `headers @> '{...}'` containment with a GIN
  index; the same JSONB is the candidate substrate for 12b's
  delays/tiers/shedding if those want to be header-driven. Foot-gun: an
  empty {} match is @>-true for EVERY event — reject at bind time. Also
  parked here: a NATS-style selector for pattern bindings — today `*`
  matches any run including dots and can't pin an exact depth; NATS splits
  `*` (exactly one token) from `>` (trailing tokens). Real systems: RabbitMQ
  header exchanges (x-match all/any).
- **LISTEN/NOTIFY latency** (8d) — producers wake idle workers instead of
  waiting for the poll tick. NOTIFY is fire-and-forget (lost if no listener
  or during reconnect), so the fallback poll stays underneath — it also
  covers delayed (run_at) messages. Same pattern River/Oban use. Revisit
  only if poll-interval latency is a measured problem.
- **Lease heartbeat/renewal** (9b) — for jobs whose legitimate runtime
  exceeds WorkTimeout but still want fast crash reclaim: an opt-in
  heartbeat()/touch() handle passed to consumerFunc; the lease extends only
  when touched (`UPDATE ... SET lease_expires_at = now()+ext WHERE id=$1 AND
  lease_token=$2`); RowsAffected==0 on renew means already reclaimed ->
  cancel the work context (the row is another worker's now — never retry the
  renew). Settled gotchas: renewal is PROGRESS-based (Temporal
  activity-heartbeat style), never an unconditional background ticker — only
  the user can tell slow-but-progressing from hung; interval ~ window/3
  survives ~2 missed beats; the extension must cover the ack, not just
  processing; keep a hard max-duration ceiling so a hung-but-touching loop
  still caps out; in-process the library can only stop renewing, never kill
  the goroutine. Prerequisites (Phase 13 boundary settle, debug readout)
  are satisfied — pick up on merit when a real long-running workload shows.
- **pgx vs database/sql** (11b) — decide whether dropping pgx for
  database/sql is worth losing native types, COPY, pgx.Batch pipelining, and
  LISTEN/NOTIFY; pgx.Tx threads through every producerFunc closure and
  pgtype.UUID sits in public structs, so the swap means re-deriving all of
  it for portability nobody asked for. Inventory the dependencies, weigh 8d
  first if it's in play, write the decision even if it stays "keep pgx".
  River and Oban both commit to pgx for the same reasons.
- **Dynamic partition bounds** (11.5b; shape settled 2026-07-24 — unlocks
  the immutable PartitionSize). Today every partition-math call site assumes
  one constant width for the stream's life. The fix: Postgres already stores
  every partition's true bounds — the math is just a cache of the catalog.
  KEEP sequential `message_log_<id>_<n>` naming; reads walk pg_inherits +
  pg_get_expr(relpartbound) and use the (relname, lower, upper) triples;
  creation mints n = max suffix + 1 and from = max upper bound off ONE
  catalog read (CREATE TABLE IF NOT EXISTS keeps the concurrent-create race
  benign — racers on the same snapshot compute identical name+bounds);
  contiguity + non-overlap survive any size history because new partitions
  only append at the top; cache the partition map in memory, re-read only
  when head crosses the cached max upper bound. Resulting semantics:
  PartitionSize = width of FUTURE partitions only (Kafka segment.bytes) —
  a freely-alterable stream-row UPDATE via AlterConfig's sparse-patch
  machinery.
- **Consumer lifecycle extension point** (13b) — decide whether the
  startup -> poll -> shutdown sequence becomes an overridable public
  Lifecycle struct or stays internal. Deferring was itself the v1 decision:
  internal for now; publishing hook points freezes the poll loop's internal
  ordering into API. Pick up only on a real embedding need (external poll
  trigger, custom scheduler, hand-driven test harness). sarama's
  ConsumerGroupHandler vs River's internal loop are the two defensible
  answers.
- **RLS & chaos-testing surfaces** (13c) — both additive post-v1. RLS: most
  likely a stream.Config toggle provisioning Postgres RLS policies + a
  least-privilege role so a compromised consumer credential can't reach
  outside its stream; decide the config field, which tables carry policies,
  role-to-stream mapping, RegisterStream-rides vs separate admin verb.
  Chaos/fixture: internal seed/inject helpers first (seed ready/inflight/
  dead, inject failures), then decide public testing package vs
  internal-only (River's rivertest precedent).
- **Presence: heartbeat rows for live producer/consumer instances** (13d;
  design shaped in discussion, not built; prerequisite for the circuit
  breaker's quorum-as-a-fraction). Nothing records what's connected to a
  stream — operators can't answer "what exists right now, idle or active",
  and Destroy finds out the hard way (a live producer's missing-partition
  self-heal resurrects partitions mid-drain).
  - One presence row per instance, three timestamps, two mechanisms:
    registered_at written once at Register (what EXISTS); last_heartbeat
    bumped by a lifetime heartbeat goroutine (what's ALIVE — crashed process
    leaves a stale row for a TTL sweep); last_produced_at/last_consumed_at
    (what's ACTIVE) bump an in-memory atomic on the hot path, flushed on the
    heartbeat tick — zero hot-path writes. Activity-only heartbeats rejected
    (collapse "nothing registered" and "registered but idle"); piggybacking
    on the Consume loop rejected (breaks symmetry, misses janitor-only
    instances).
  - Register(ctx) inserts the row, validates the stream's PARENT tables via
    to_regclass (parents only — partitions come and go by design), starts
    the heartbeat. The shipped three-state gate keeps presence honest:
    producing implies alive becomes an invariant.
  - First consumer — a Destroy gate: refuse while any producer is ALIVE (not
    merely active; idle-but-alive can wake mid-drain), refusal naming
    instances and last-seen times; force override; deleteStream's bounded
    drop loop stays the hard backstop (check-then-drain has an unavoidable
    TOCTOU window). RabbitMQ queue.delete(if-unused) precedent.
  - Second consumer — the breaker's globalization quorum as a fraction of
    ALIVE instances. Also the natural substrate for alerts that name
    instances ("destroy blocked: producer X seen 2s ago").
  - Third consumer — automatic consumer-group expiration
    (stream.Config.GroupExpiration): janitor-style reap of a group idle past
    the threshold, deleting the same rows as the manual destroy verb.
    Mechanism settled 2026-07-29: idleness computed DYNAMICALLY —
    now() - GREATEST(newest heartbeat, group registered_at) — never a
    recorded became-empty transition (missable on crash; derived form is
    idempotent). Idleness keys on MEMBERSHIP (heartbeats), never activity
    timestamps — Kafka (KAFKA-4682) and Pulsar (#17573) both shipped GC
    that deleted state under live-but-quiet groups and fixed it by anchoring
    to membership. Prerequisites: newest heartbeat must SURVIVE instance
    departure (retain last row per group, or roll last_seen onto the
    consumer_group registry row); never-consumed groups floor at
    registration time. Default open question: retention-anchored
    max(RetentionTTL, 7d) vs industry-standard OFF (Pulsar/NATS opt-in;
    Kafka the lone always-on at 7d) — re-settle at build; needs an explicit
    never value (0-as-unset vs 0-as-never collide). Retention-forever
    streams never expire groups. Expiry is recoverable by design: a returning
    group re-seeds and REPLAYS what retention holds — duplicate work, not
    data loss. Stakes: with allow_drop_past_committed=false (default) an
    abandoned group's cursor pins partition drops. Hard rules: expiry must
    telegraph (idle/expiring visible in worker snapshots/state gauges
    BEFORE the reaper), and unresolved delivery rows refuse-or-alert, never
    silent drop.
  - Fourth consumer — the producer half of the stop-line session summary
    ([0567]): the producer has no lifecycle, so its produced counter's
    stopped line is the heartbeat goroutine's stop. The consumer side is
    not gated here — its session counters flush under a session uuid.
  - Real systems: RabbitMQ if-unused; Kafka group membership
    (session.timeout.ms as the TTL sweep); Temporal worker pollers view.
- **Post-v1 research backlog** (14d):
  - Contribute a MIN_ACTIVE_ROWVERSION-style primitive upstream to Postgres
    (watch-and-propose only): SQL Server's MIN_ACTIVE_ROWVERSION() is a
    cheap first-class read of the low-water mark across in-flight
    transactions — exactly what the snapshot fence
    (pending_head/pending_xid cursor columns) answers by hand. A core
    primitive would let claim fences poll a system value instead of carrying
    tracking columns.
  - Read for hot-path ideas once the API stops moving:
    https://packagemain.tech/p/golang-optimizations-for-highvolume — mine
    for the actual hot paths (claim, produce, janitor loops), don't apply
    speculatively.
  - worker_run_log failure-evidence table (renamed from worker_log by
    [0570], which reserves that name for worker metadata history; design
    settled under the old maintenance-tier names; rides the worker
    backoff's fenced failure UPDATE): one SHARED append-only table, failed
    worker runs only —
    `worker_run_log (id BIGSERIAL PK, worker, stream_id, consumer_group,
    error TEXT, attempts INT, created_at)`; NO success/recovery rows
    (absence IS success). The write rides the backoff UPDATE's fence as one
    data-modifying CTE, so an instance that lost its claim mid-run can't
    write noise. Retention: the janitor sweeps rows past ~7d in its
    existing pass. Surfacing: worker snapshots join the latest log row per
    failing worker.
- **Claim-fence transaction-xmax logic -> its own async ticker** (really
  want) — abstract the fence read into an async ticker with claimers
  reading a shared in-memory value; the query is cheap so the poll rate can
  be much faster, and the complex logic gets one home.
- **Exception claiming revamp onto stream/cursor machinery** (really want) —
  exception claiming is queue-based today and carries a lot of custom logic.
  Converting it needs the async ordered-index claim table (see 12b): a
  failed retry is produced to an unordered stream, materialized as a new
  attempt near the front of the ordered index, picked up soon after because
  materialization runs only slightly ahead of claiming.
- **Delivery rows delete on completion** instead of persisting as 'done' —
  the exception queue's irreducible job is a dispatch index over pending
  messages, not a completion record; deleting on success makes storage
  O(pending window) instead of O(history). Composes with success-by-absence
  audit: "was message X processed by group Y" = id <= the group's frontier
  AND no dead row AND no open exception, with delivery_log supplying the
  attempt history — per-message audit with zero happy-path writes. Caveats:
  no success timestamp to report; audit horizon = retention TTL.
- **Page-bitmap completion tracking** as a someday replacement for range
  leases on the cursor path: a page row per N ids above the committed
  cursor holding a done-bitmap gives bit-granular crash recovery (reclaim
  redelivers only unset bits), updates batched one page-row UPDATE per
  commit — the structure Pulsar keeps for individually confirmed messages
  below its cursor. Deliberately NOT a route to custom
  dispatch order (bitmaps compress "who's done"; priority/delay/fairness
  need "who's next", one index entry per pending message regardless).
  Only worth building if range-granular crash redelivery shows up as a real
  cost — it's rare-path today.
- **json.Number sweep for jsonb-through-Go paths** — any jsonb that
  round-trips through a Go `map[string]any` (insertWorker's metadata merge is
  the known case) decodes numbers as float64, silently corrupting integers
  above 2^53 (~104 days in nanoseconds) on write-back. Fix is decoding with
  `json.Decoder.UseNumber()` (scan the jsonb as bytes, decode both maps
  ourselves); the merge itself never touches values, so digit-strings pass
  through lossless. Do it as one audited sweep of every such path, not a spot
  fix — pre-v1 the realistic values sit far below the threshold.
- **Partition delivery_<id> + delivery_log_<id> by message_id range** on
  message_log's bounds ([0572]) — the janitor's partition-drop cleanup runs
  range DELETEs through both tables today; aligned partitions turn those
  into drops. Benchmark-gated: create-ahead must ride the producer's
  partition self-heal, and the hot claim table takes on partitioned-table
  planner overhead.
- **stream_config_log / worker_config_log retention** — both are unbounded
  today; rows append only on actual config change, so growth tracks change
  frequency, not traffic. Revisit whether they want a TTL sweep like
  binding_config_log's ([0573]) once real deployments show the volume.
- **BRIN indexes** — look into using them for different tables.
- **DeadLetterStream consumer** — consume on events to the DLQ.
- **Shadow/Mirror functionality** — watch exactly the same cursor as another
  group (message-by-message mirroring would be better if possible; probably
  not).
- **Binary/compressed message storage** (maybe as an option) — prevents easy
  field search in the DB but could mean smaller network requests and faster
  unmarshalling, protocol-buffers style. https://github.com/Apaezmx/pgproto
- **Debezium-like generalization** — how could the system watch a generic
  table and stream its data elsewhere.
- **Standardized HTTP API design** — producing is a POST with batching first
  class; consuming via SSE/websockets needs a cursor-advance
  acknowledgement mechanism, or a plain GET/QUERY as the simple
  alternative. https://github.com/durable-streams/durable-streams for a
  potential protocol.
- **WorkerManager split into WorkerScheduler + WorkerSpawner** — scheduler
  acts like the schedule producer but submits 'spawn'/'destroy' stream requests
  (reconciler logic); spawner reads the stream and spawns or destroys
  instances.
- **Antithesis** (https://antithesis.com) for production hardening and bug
  hunting.
- should look into using https://go.dev/doc/go1.27#goroutineleak-profiles instead
  of potentially our custom goroutine tracking - might simplify things / make it
  easier for users as well
- should make more use of vale for standardized writing style its an interesting idea

- **Agent operator** -- a skill set that lets a coding agent (Claude Code
  or similar) diagnose a running deployment through the existing CLI.
  Delivery is skill files over `sqlstreams --output json` and `sqlstreams explain`,
  not an MCP server: users drop MCP servers for CLI-plus-skills on token
  cost and keep MCP only for auth brokering or a hard allowlist, and the
  allowlist here is a read-only Postgres role. Read-only by construction:
  the operator runs read verbs, prints the write command (suspend, run,
  migrate) for a human to paste, never runs it. Every read carries a scope
  (stream, consumer, limit). A diagnosis ends by naming the alert or metric
  that should clear and re-reading it. Rung 0 before building: point an
  agent at a broken e2e test deployment with only the CLI and record where it
  goes wrong; SQL codes, fix text, and alert hints may already be enough.
  Evidence 2026-09-07: Supabase MCP exfiltration and Kiro prod delete
  (credential scope, not prompts); kubectl-ai #628 (no-execute mode);
  HolmesGPT #2438 (unscoped reads); Confluent MCP hands-on (fell back to
  the CLI); Palark k8sgpt eval (generic fixes).
