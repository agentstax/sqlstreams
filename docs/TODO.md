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

- [ ] Separate measurement from condition comparison inside the existing
  partition-count controller, only as needed by real callers. Keep one source
  read and one comparison shared with immediate registration-time warnings.
- [ ] Record healthy and unhealthy raw partition counts through the alert
  worker's existing metrics producer; reuse the retained rank-window read.
  Resolve policy once per check from the current schedule, preserving raw
  evidence for later threshold changes.
- [ ] Extend Record/classify to evaluate retained history after the alert-head
  lock. Derive consecutive duration there; do not add a second classification
  pipeline or manufacture an active Alert before deciding to record one.
  Healthy evidence may resolve; pending/insufficient evidence must not.
- [ ] Add only the config/result fields consumed by this complete path. Keep
  pending off by explicit choice, freshness/gap/window validation, and no
  pending timer or cursor. Use database observation/evaluation time.
- [ ] Add the approved read-only diagnostic through the existing alert handle
  using the same evaluation path; retain Latest/History's recorded-message
  contracts. Do not add a persisted status mirror.
- [ ] Verify spikes, sustained conditions, recovery, overnight gaps, late/tied
  observations, retention, policy changes, and concurrent/retried recording.
  Run targeted checks and affected labs; measure the existing queries before
  changing hourly defaults to the proposed one-minute cadence.

### 5. Adapt the other conditions, then collector progress

- [ ] Resolve compaction applicability/pair identity and worker-detail evidence
  in the existing condition implementations before coding their adoption.
  Preserve current alert messages and topic-scoped worker meaning; do not
  invent a generic snapshot store or split condition facts across ambiguous
  samples to satisfy a preselected measurement shape.
- [ ] Apply the partition-count recording path to compaction read cost and
  worker liveness. Worker checks keep independent snapshot reads; their
  ability to observe trouble cannot depend on the metrics collector.
- [ ] Record full-pass collector completion after all writes succeed; startup
  and partial passes do not refresh it. Add independent scheduled progress
  observations and use the same Record/classify path, with no extra runner.
- [ ] Verify each adaptation's real worker, diagnostics, restart behavior, and
  repeat/recovery semantics with targeted checks and affected labs.

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

- [x] Settle the ledger row shapes: append-only facts, key `<producer>-<seq>`,
  two produce rows (attempted, outcome) and one handler row per invocation;
  `produce_ledger`, `handler_ledger`, `run_phase` share the JSON-lines names.
- [x] `Scenario` struct with `quiet` and `dev` declarations; a String printer
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
