# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

## Metrics export and history-based alerts [0682] [0683] [0686]

Implement the approved direction in
`website/src/content/docs/concepts/metrics-export.mdx`. Each chunk is a review
checkpoint, not permission to expand scope. Use the smallest change to existing
machinery. No separate pending-state table, registration/discovery cache,
generic per-series freshness companion, or new backlog alert.

During code iteration, defer alert-lab runs and repairs to the review checkpoint
at the user's request. Use targeted compile and unit checks while editing.

Current priority: complete the same pending capability for all three existing
alerts before diagnostics or OTel work. Partition-count support alone is not
this checkpoint's completion condition.

### 1. Settle the remaining implementation contract

Original collector-only contract approved in [0684], now superseded by [0686]
for shared alert support. The completed checklist below records that original
review; chunks 3–5 replace the rolled-back implementation plan. Runtime remains Proposed in
`concepts/metrics-export.mdx` and `concepts/alert-history.mdx` until shipped.

- [x] Check current collector, history reads, schedule resolution, alert
  classification, retention, and transaction/compaction-head seams.
- [x] Draft concrete names, raw timestamp observations with explicit absence,
  one-minute checks, age/pending/gap defaults, sufficient history windows,
  and serialized alert production through the existing head.
- [x] Review public health/diagnostic names and the representation of collector
  completion and progress observations, including explicit absent evidence.
- [x] Set check cadence, progress thresholds, pending duration, allowed gaps,
  and freshness rules. Define sufficient history windows, boundary evidence,
  retention requirements, and deterministic ordering of late/tied observations.
- [x] Specify activation, recovery, repeats, and concurrent/repeated evaluation
  guarantees separately from deterministic calculation. Identify which existing
  alert checks, if any, adopt history-based evaluation; preserve the others.
- [x] Update the Proposed page with these choices for review before affected
  code; record newly settled decisions in the same session.

### 2. Read sufficient retained measurement history

Originally implemented a rank-window read. Under [0690], this is now
`CompactionController.ListKeyMessagesByCreatedAt`, reached through metrics'
`GetMeasurementHistory`, which supplies database time and validates retention.
The unused rank-window method is removed.
Each history read owns its complete SQL; row scanning and payload decoding
are shared. No supported public API or table changes; the evaluator supplies
the policy's time bounds.

- [x] Extend the existing core history-read path to supply the agreed bounded
  time window and boundary evidence. A fixed row limit is not proof of duration.
  Keep alert policy out of SQL and preserve existing public history behavior.
- [x] Make observation-time ordering and incomplete/expired evidence explicit.
  Return retained rows only, preserving gaps and errors; policy validation
  belongs with the shared evaluator in chunk 4.
- [x] Verify window boundaries, tied/late observations, retention gaps, and
  existing history callers with targeted datastore/controller integration tests.
  Build, vet, targeted race tests, the PostgreSQL retained-history integration
  test, and `just metrics-collector-lab` passed. No full-suite checkpoint yet.

### 3. Make existing alert recording atomic

The standalone history implementation was rolled back. No shared evaluation
prototype is complete. Start where all three existing checks already meet:
`instance.evaluateTopics -> condition.Evaluate -> AlertController.Record ->
classify -> alert producer`. Keep condition evaluation and startup warnings
unchanged in this task.

- [x] Put the existing head read, classify call, and alert production in one
  transaction using LockHead and ProduceInTx. Log transitions only after
  commit. Keep the current repeat, severity-change, and recovery rules.
- [x] Update the three existing worker call sites, without another runner,
  evaluator hierarchy, observation-status model, or supported API change.
- [x] Verify concurrent activation/recovery, quiet checks, repeat handling,
  corrupt-head isolation, and committed-only logs in the existing alert lab.
  Targeted build/vet/race checks and the alert lab passed, including rejected
  writes leaving both alert history and transition logs unchanged. Docs
  lint and site build passed. No full-suite or fresh-database run.

This serializes recorded transitions, not source observations taken before
Record. Quiet checks also lock/create the head identity. Transaction errors
return to the existing worker retry path; no automatic transaction replay is
added. History ordering and database evaluation time belong to chunk 4.

### 4. Integrate partition-count history in that recording path

Deliver a working existing alert, not an unconnected general calculator.
The site remains the behavior proposal; names or abstractions from the
rolled-back prototype are not implementation requirements.

- [x] Metrics collection owns partition-count measurements. Add the count to
  TopicSnapshot and the existing collector; expose the normal topic metric.
  Remove alert-specific evidence production and AlertEvaluation [0693].
- [x] Partition-count Evaluate reads the collector's retained measurement;
  Record only records alerts. Missing measurements fail evaluation instead of
  resolving an alert. Registration warnings use the same read-only evaluation.
  Build, vet, and targeted unit/race checks passed; lab adaptation and execution
  remain deferred. Pending and freshness-window evaluation are not enabled.
- [x] Add the retained CreatedAt-window read through metrics, without a fixed
  row limit. Use inclusive bounds and created_at/id descending order; include
  superseded messages. Remove the unused rank-window path. Existing count-limited
  History behavior is unchanged. Database boundary/order checks remain deferred
  to the lab checkpoint; unit checks cover invalid bounds before I/O.
- [x] Use the consumed schedule payload as the policy for that run. Schedule
  changes affect subsequently produced messages, not queued checks. Preserve
  raw evidence for evaluation under the supplied threshold.
- [x] Apply AlertEvaluationResult consistently to every evaluator, scheduled
  worker, and registration warning [0694]. Record accepts explicit healthy,
  pending, active, or insufficient evidence; only healthy can resolve. Pending
  and insufficient results never access the alert head. Targeted race tests,
  vet, and conventions pass.
- [x] Extend existing evaluation to derive consecutive duration from collected
  history [0695]. Keep Record/classify responsible for serialized alert transitions.
  Healthy evidence may resolve; pending/insufficient evidence must not.
- [x] Add only the config/result fields consumed by this complete path. Keep
  pending off by explicit choice, freshness/gap/window validation, and no
  pending timer or cursor. Use StoredMessage.CreatedAt for evidence timing
  and database time for evaluation; accept delayed writes as fresh evidence.
  Partition count uses the consumed timing policy; all evaluators accept the
  same payload. Other conditions retain live reads until chunk 5. Insufficient
  evidence increments failed-topic counts; registration warnings remain immediate
  with fresh evidence. Targeted race tests cover duration, gaps, ties, stale and
  unusable evidence, disabled pending, and replayed calculations. Database
  window/retention checks and alert-lab repairs remain deferred to the checkpoint.
- [ ] Verify spikes, sustained conditions, recovery, overnight gaps, late/tied
  observations, retention, policy changes, and concurrent/retried recording.
  Run targeted checks and affected labs; measure the existing queries before
  changing hourly defaults to the proposed one-minute cadence.

### 5. Adapt the other conditions, then collector progress

- [x] Resolve compaction applicability/pair identity and worker-detail evidence
  in the existing condition implementations before coding their adoption.
  Preserve current alert messages and topic-scoped worker meaning; do not
  invent a generic snapshot store or split condition facts across ambiguous
  samples to satisfy a preselected measurement shape.
  Measurement.Metadata is approved and implemented [0698]: compaction status
  accompanies the partition count; topic-scoped unclaimed-worker measurements
  carry the matching worker identities. No compatibility machinery or size limits.
- [x] Move the duration calculation into the shared alert domain;
  all three conditions interpret their evidence for that same calculation.
  Keep Evaluate -> Record and expose PendingDuration, MaximumGap, and DisablePending consistently on their configs.
- [x] Apply collected-history evaluation to compaction read cost and worker
  liveness after the required evidence is collected by metrics. Collector
  progress monitoring must remain independent of the collector it monitors.
  All three now read retained measurements, share evaluation.EvaluateHistory, and carry
  the same timing fields in their consumed policy. Old compaction live-read SQL is removed.
  Targeted race tests cover shared timing, each condition, metadata, collector
  topic/group ownership, healthy zero samples, and all three schedule configs.
  OTel's existing integration test now asserts metadata is not exported; it
  remains database-gated. No labs or compatibility runs during this change.
- [x] Record full-pass collector completion after all writes succeed; startup
  and partial passes do not refresh it. The final measurement uses the existing
  produce path and rank-zero compaction. VK0100 and the system metric selector
  expose the completion timestamp. Database failure-path checks remain deferred.
- [x] Add append-only worker_instance_log snapshots in the existing datastore
  mutation transactions, with TTL cleanup [0700]. Preserve instance identity,
  original creation time, and expiry without lifecycle-operation fields.
  History survives live-instance deletion; release keeps last-expiry semantics.
  Manager cleanup retains snapshots for 24h after recorded expiry by default.
  Targeted PostgreSQL race test passed for snapshot writes, claim/renew rollback,
  declined/lost claims, release/expiry survival, retention, and system deletion.
  Build, vet, and conventions passed. No labs run.
- [ ] Read manager lease coverage over the evaluation window, including boundary
  evidence. Combine it with collector completion history for progress evaluation.
  Preserve pending across continuous replacements and break it across gaps.
  Use the existing scheduled check and Record/classify patterns; no independent
  progress-observation series or separate observation worker.
- [ ] Verify each adaptation's real worker, diagnostics, restart behavior, and
  repeat/recovery semantics with targeted checks and affected labs.
- [ ] After all three existing alerts support pending, add the approved
  read-only diagnostic through the existing alert handle using the same
  evaluation path. Retain Latest/History's recorded-message contracts; no
  persisted status mirror.

### 6. Replace instrument registration with the OTel producer

- [ ] Implement the external SDK producer over current core measurement reads,
  removing registration/discovery and cached topic identity. Preserve supported
  gauge/counter meaning, units, attributes, and explicit resource ownership.
- [ ] Apply one portable family-validation policy using the upstream translator:
  conflicting name/kind/unit families, translated metric/attribute collisions,
  and reserved outputs. Export healthy families with current-collection rejection
  diagnostics; do not invent counter start times or freshness companions.
- [ ] Return source-read health 1 for successful reads, including empty results;
  on failure return health 0 plus the error, without retained measurements or a
  rejection count. Keep collection bounded by context/timeout.
- [ ] Verify conversion, empty/error reads, naming conflicts, recovery after
  rejection, refreshed topic lookup, and concurrent collections in the nested
  OTel module. Update affected public references/examples with the API change.

### 7. Wire readers, Prometheus, and lifecycle ownership

- [ ] Attach the producer to supported SDK readers and retain the upstream
  Prometheus exporter. Migrate callers and remove the obsolete registration
  path; keep provider shutdown and caller-owned database pool ownership explicit.
- [ ] Verify ManualReader/Prometheus preserve health data alongside source errors
  (a scrape may return HTTP 200). Verify PeriodicReader skips export on error;
  document a dedicated Vulkan periodic pipeline so application metrics remain
  independent of Vulkan source failures.
- [ ] Test repeated/concurrent scrapes, cancellation/timeouts, in-flight shutdown,
  and recovery. Check existing deployment guidance for a single logical export
  target without adding a new coordination mechanism.

### 8. Verify the complete behavior and close out

- [ ] At each chunk, run targeted builds, `go test -race` for touched packages,
  and directly affected labs. At the review-ready checkpoint, run the full
  fresh-database lab suite; do not run it after every chunk.
- [ ] Check the implemented behavior against the proposal and every finding in
  `OTEL_REVIEW.md`, applying current conventions where the review is stale.
  Confirm existing alerts adopt only the reviewed timing/evidence changes,
  and metric callers retain their contracts.
- [ ] Update site examples and diagnostic references as behavior ships; remove
  Proposed labels only for verified implementation. Run relevant docs checks
  and the site build, then record the shipped milestone in HISTORY.md.
- [ ] Fold resolved review findings into the fixed record-keeping surface before
  removing `OTEL_REVIEW.md`; remove completed TODO/ROADMAP work at close-out.
  If this is a release checkpoint, also run prior-tag compatibility verification
  and update the migration table and release history with its outcome.
