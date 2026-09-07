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

### 1. Settle the remaining implementation contract

Original collector-only contract approved in [0684], now superseded by [0686]
for shared alert support. The completed checklist below records that original
review; chunk 3 reopens the affected choices. Runtime remains Proposed in
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

Implemented `CompactionController.ListKeyMessagesByRank`: the new built-ins'
`At.UnixMicro()` ranks bound their history without measurement-aware SQL.
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

### 3. Revise the shared alert contract — next review checkpoint

Reopened after the collector-only calculation review [0686]. The existing
unconnected implementation and tests are useful starting material, not a
completed shared capability. Step 2 stays complete; runtime code has not
changed during this proposal revision.

- [x] Revise the Proposed alert-history page around shared duration evaluation
  and the existing AlertController.Record path. Show partition-count pseudocode
  and a concrete sequence; align metrics-export and record the scope change.
- [x] Draft shared AlertPendingConfig (Duration, MaximumGap, Disabled),
  proposed cadence/defaults, current-schedule policy resolution, immediate
  mode, and query-cost checkpoints. Preserve registration-time log-only checks.
- [x] Draft per-owner measurement names, attributes, time/rank ordering,
  healthy/unusable evidence, and partition-count flow using existing reads.
  Counts alone cannot reconstruct worker details; tied compaction pairs need
  an identity rule. These are explicit open design gates, not implementation
  details to patch around.
- [x] Propose AlertHandle.Snapshot using the same calculation, exposing status,
  observation time, supported span, and resolved timing without a persisted
  state mirror. Existing Latest/History retain active/resolved semantics.
- [ ] Review config/API names, default cadence and tolerance, and the snapshot
  contract. The page's one-minute/two-minute settings remain unapproved.
- [ ] Resolve compaction pair identity and worker detail retention before
  declaring the full shared evidence contract complete. Do not add a generic
  snapshot store or silently trim existing alert messages to close these gaps.
- [ ] Approve that contract before code; record additional settled choices.

### 4. Prove the shared path with partition count

- [ ] Move the general duration/gap/freshness calculation and tests into the
  existing alert controller. Keep collector timestamp interpretation local;
  do not create another duration mechanism or persist pending state.
- [ ] Adapt the existing partition-count check to retain raw observations,
  including healthy counts, then invoke shared history evaluation. Resolve
  condition policy once per run; changed thresholds reuse raw evidence.
- [ ] Validate the sufficient window and retention, reusing step 2's read.
  Preserve fixed-history determinism, tied/late observation ordering, and
  actual execution timestamps rather than scheduled timestamps.
- [ ] Extend existing alert recording/repeat handling to consume the result.
  Pending/insufficient evidence must never become nil-means-healthy recovery.
  Serialize decisions through the existing head and produce-transaction seam;
  refresh evidence after locking. Read errors do not fabricate observations.
- [ ] Add the agreed diagnostic visibility and verify brief spikes, sustained
  conditions, fresh recovery, stale/missing evidence while active, overnight
  gaps, policy changes, repeats, concurrent checks, and ambiguous commits.
- [ ] Run targeted build/vet/race checks and directly affected labs, including
  query-cost checks for the proposed cadence; review this complete existing
  alert before adapting the others. No full-suite checkpoint yet.

### 5. Adopt the shared path for the remaining checks

- [ ] Adapt compaction read cost and worker liveness without changing their
  condition meaning. Preserve applicability and topic-scoped worker identity;
  worker checks keep their own existing snapshot reads, independent of the
  metrics collector. Review per-check evidence and defaults before adapting.
- [ ] Record collector completion only after the full pass and all concurrent
  writes succeed. Startup and partial passes do not refresh completion.
- [ ] Add collector progress through existing scheduled alert machinery:
  independently record the completion seen or explicit absence, then use the
  same shared evaluation/recording path. No separate runner or user state.
- [ ] Verify all checks' activation, recovery, gaps, and diagnostic visibility;
  test collector partial failure, observer independence, and whole-system
  restart. Run targeted checks and affected labs per adaptation.

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

## Reliability lab v1 [0687]

Build the simple case under `bench/reliability/` against the Proposed page
`website/src/content/docs/concepts/reliability-lab.mdx`. One plain topic, one
group, fail rate 0, constant rate, fixed instance count. Smallest delta to the
bench module (pgx only); no new dependency.

### 1. Ledger shape and scenario declaration

- [ ] Settle the ledger row shapes (produce attempted/outcome, handler
  invocation, run_phase) as one JSON-lines format and the `lab` schema tables.
- [ ] `Scenario` struct with `quiet` and `dev` declarations; a String printer
  emitting the `.scenario` format; a test diffing it against the checked-in
  file.

### 2. Roles and compose

- [ ] One binary, `-role producer|consumer|checker`, `-scenario`, `-time-scale`.
- [ ] Producer: open-loop constant-rate pacer, latency from scheduled time,
  two ledger facts per produce, idempotency key per attempt.
- [ ] Consumer: N in-process instances, handler writes one ledger fact per
  invocation, `DeliveryLogModeAll`.
- [ ] Compose: Postgres with healthcheck, one image, `--scale consumer=N`, a
  volume for ledger files, no `restart: true` on dependents; `just
  reliability-lab scenario=dev`.

### 3. Checker and report

- [ ] COPY ledger files into the `lab` schema; drain until each producer's
  last committed key has a delivery outcome, budget-bounded.
- [ ] Checks: committed == message_log rows; every message >= 1 delivery
  ending success or dead; bucket sum; duplicates counted; reclaims == 0;
  dead == 0. Verdict pass/fail/unknown, exit 0/1/2/3.
- [ ] Record JSON to `results/<scenario>/<timestamp>/` plus the scenario
  printed back with actuals; carries `synchronous_commit` and build version.

### 4. Verify and close out

- [ ] `dev` green for one minute; then sabotage: delete a message_log row and
  drop a handler ledger line, confirm each fails; confirm a run with zero
  produced reads unknown.
- [ ] HISTORY.md entry citing [0687]; remove this section and the ROADMAP
  pointer's v1 line; delete `reliability-lab-research.md` at repo root.
