---
status: accepted
date: 2026-09-11
phase: "pre-v1"
---

# Published throughput uses three fixed-duration runs and retains every outcome

## Context

The aggregate-recording lab recovered the native scratch throughput. The
user approved collecting repeatable evidence for the README and docsite,
including making CPU headroom informational for maximum-throughput runs.

## Decision

- Run max-throughput three times from one frozen source and binary. Each
  run has five minutes of warmup and 25 minutes of measured production.
- Use the lower producing/consuming rate from each measured hold; publish
  the median of the three rates and their full range, never the best minute.
- Preserve all attempts and original verdicts. Backlog, application errors,
  and storage limits remain required; do not discard poor results to replace
  them with better runs. Report maintenance retries alongside performance.
- Set max-throughput's generator_headroom expectation to report. Phase
  summaries respect that expectation while still retaining the breach count.
  Other scenarios retain their existing policy; historical verdicts stay intact.
- Keep [0747](0747-throughput-scenarios-can-disable-message-recording.md)'s
  aggregate evidence and its limits. No exactly-once or detailed-latency claim.
- Keep selected evidence under .bench/reliability/results/published: source,
  build hashes, declarations, effective settings, records, logs, and verdicts.
  A static docsite report links evidence; the README links that report.

## Consequences

- A headline is tied to a concrete workload and host, not a universal ceiling.
- The three runs take approximately 90 minutes and share the 100 GB storage
  budget; each owned database is dropped before the next run starts.
- No new reporting service, site build pipeline, or library API is required.
