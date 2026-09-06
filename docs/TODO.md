# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

## Metrics export and history-based alerts [0682] [0683]

Implement the approved direction in
`website/src/content/docs/concepts/metrics-export.mdx`. Each chunk is a review
checkpoint, not permission to expand scope. Use the smallest change to existing
machinery. No separate pending-state table, registration/discovery cache,
generic per-series freshness companion, or new backlog alert.

### 1. Settle the remaining implementation contract

Proposed choices are drafted in `concepts/metrics-export.mdx` and
`concepts/alert-history.mdx` under `website/src/content/docs/`. Awaiting user
review before recording them as accepted or starting chunk 2.

- [x] Check current collector, history reads, schedule resolution, alert
  classification, retention, and transaction/compaction-head seams.
- [x] Draft concrete names, raw timestamp observations with explicit absence,
  one-minute checks, age/pending/gap defaults, sufficient history windows,
  and serialized alert production through the existing head.
- [ ] Review public health/diagnostic names and the representation of collector
  completion and progress observations, including explicit absent evidence.
- [ ] Set check cadence, progress thresholds, pending duration, allowed gaps,
  and freshness rules. Define sufficient history windows, boundary evidence,
  retention requirements, and deterministic ordering of late/tied observations.
- [ ] Specify activation, recovery, repeats, and concurrent/repeated evaluation
  guarantees separately from deterministic calculation. Identify which existing
  alert checks, if any, adopt history-based evaluation; preserve the others.
- [ ] Update the Proposed page with these choices for review before affected
  code; record newly settled decisions in the same session.

### 2. Read sufficient retained measurement history

- [ ] Extend the existing core history-read path to supply the agreed bounded
  time window and boundary evidence. A fixed row limit is not proof of duration.
  Keep alert policy out of SQL and preserve existing public history behavior.
- [ ] Make observation-time ordering and incomplete/expired evidence explicit.
  Use the agreed retention policy rather than inventing missing observations.
- [ ] Verify window boundaries, tied/late observations, retention gaps, and
  existing history callers with targeted datastore/controller integration tests.

### 3. Calculate alert state from history

- [ ] Implement the domain calculation over fixed history, evaluation time,
  and policy: healthy, pending, active, or insufficient evidence. Reuse one
  calculation for sustained conditions; do not persist a pending timer/cursor.
- [ ] Measure the consecutive unhealthy sample span, stopping at a healthy
  observation or excessive gap. An old sample cannot become active by waiting.
  Only fresh healthy evidence permits recovery.
- [ ] Verify threshold boundaries, stale/missing evidence, overnight gaps,
  evaluator restarts, and identical results for identical inputs.

### 4. Record collector completion and independent progress evidence

- [ ] Record successful completion only after the entire collection pass and
  all its writes succeed. Preserve prior completion evidence across startup;
  partial writes or failed passes must not count as completion.
- [ ] Use existing core scheduled alert machinery to independently record raw
  progress observations: observation time and last completion seen, or explicit
  no retained completion evidence. A failed read produces no fabricated sample.
- [ ] Evaluate each historical progress observation against its own observation
  time. Reuse chunk 3; do not add a second duration mechanism or user-run service.
- [ ] Verify partial collection failure, absent completion, observer operation
  while collection stalls, and restart after the whole system was stopped.

### 5. Connect history evaluation to actionable alerts

- [ ] Integrate the agreed checks with existing alert recording and repeat
  handling. Pending/insufficient evidence must not enter the existing
  nil-means-healthy path and accidentally resolve an active alert.
- [ ] Implement the agreed repeat/concurrency guarantees using existing seams
  where possible. Deterministic evaluation alone does not deduplicate messages;
  review any newly discovered design gap before adding machinery.
- [ ] Verify activation, fresh-evidence recovery, unknown evidence while active,
  repeats, replay, and concurrent evaluation. Include an overnight shutdown and
  restart: the gap earns no pending time. Users need no timers or startup state.

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
  Confirm no unintended behavior changes to existing alerts or metric callers.
- [ ] Update site examples and diagnostic references as behavior ships; remove
  Proposed labels only for verified implementation. Run relevant docs checks
  and the site build, then record the shipped milestone in HISTORY.md.
- [ ] Fold resolved review findings into the fixed record-keeping surface before
  removing `OTEL_REVIEW.md`; remove completed TODO/ROADMAP work at close-out.
  If this is a release checkpoint, also run prior-tag compatibility verification
  and update the migration table and release history with its outcome.
