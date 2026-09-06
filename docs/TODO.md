# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

## Metrics export and history-based alerts [0682] [0683]

- Proposal updated in `website/src/content/docs/concepts/metrics-export.mdx`:
  external OTel producer, source-read health, portable family validation,
  history-derived pending, and independent collector-progress observations.
  No separate pending-state table or generic per-series freshness companion.
- Review public health names, progress-observation representation and absent
  evidence, check cadence/thresholds/gaps, and sufficient history-window reads
  with deterministic ordering before implementation. Keep alert repeat and
  concurrency guarantees separate from deterministic evaluation.
- Then implement through existing core reads, collection and alert machinery;
  replace registration/discovery with the SDK producer and keep provider/pool
  ownership explicit. Users consume active/resolved alerts without timers.
- Verify targeted race/integration checks, including source errors, empty
  sources, naming conflicts, recovery, downtime gaps, and evaluator restarts;
  close out `OTEL_REVIEW.md` after its findings are resolved and recorded.
