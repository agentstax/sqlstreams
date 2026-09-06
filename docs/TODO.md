# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

## Metrics export review [0678]

- Review `website/src/content/docs/concepts/metrics-export.mdx`; direction
  accepted, detailed contract still proposed.
- Settle freshness representation, discovery/shutdown API, unexportable
  custom measurement handling, and deployment scope before implementation.
- Then address registration concurrency and recovery, delegate measurement
  reads to core, and align configuration ownership with CONVENTIONS.md.
- Verify the resulting contract with targeted race and integration checks;
  close out `OTEL_REVIEW.md` after its findings are resolved and recorded.
