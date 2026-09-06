---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Metrics export uses an OTel producer and source-read health

## Context

[0678](0678-metrics-export-distinguishes-read-success-and-observation-freshness.md)
proposed HTTP 5xx on source failure and discovery/callback lifecycle APIs.
The SDK external Producer can return retained measurements without instrument
registration. Its Prometheus exporter can retain producer data despite a
collection error; its periodic reader skips export on that error.

## Decision

- Supersede [0678]. Keep collection and history in core and conversion in
  otelvulkan. Each SDK Produce call uses core's existing bounded read and
  collection-local conversion; no instrument registry, discovery loop,
  retained measurement cache, or producer start/stop loop.
- Preserve gauges and cumulative totals. The caller owns its pool and
  provider; the convenience exporter closes its private provider. Recommend
  a dedicated periodic pipeline because a read error skips its export.
- Keep the upstream Prometheus exporter. Return source-read success 0 with
  the read error, or success 1 after a successful read, including an empty
  source. HTTP 200 alone does not establish source-read success.
- Both entry points enforce one portable naming contract using upstream
  underscore translation with suffixes. Reject conflicting metric families,
  translated name/attribute ambiguity, and collisions with reserved output
  names. Export healthy families plus a current-collection rejection count
  and diagnostic. Omit the count when the source could not be read.
- Do not add generic per-series freshness companions or universal expiry.
  Core retains Measurement.At; collector progress and actionable alerts use
  the history-based direction in [0683](0683-alert-pending-duration-is-derived-from-measurement-history.md).

## Consequences

New names appear on the next collection without caller-managed discovery.
OTLP-only callers accept the portable naming restrictions. External producer
data bypasses instrument Views and normal aggregation. Public health names,
conversion edge cases, and implementation remain under review on the
Proposed concepts/metrics-export page; this record does not mark code shipped.
