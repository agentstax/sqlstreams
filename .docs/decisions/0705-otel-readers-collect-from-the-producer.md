---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# OTel readers collect from the producer

## Context

The producer direction [0682] is approved. Observable instrument registration
duplicates measurement discovery and retains topic identity across collections.
This chunk replaces that path; portable rejection and health remain subsequent
work under the existing plan.

## Decision

- Metrics implements the SDK Producer contract through Produce(ctx).
  Every call reads retained measurements with CollectTimeout, resolving the
  metrics topic through core. No instrument registration, callback, or cache.
- Move the existing list read into MetricsController.ListMeasurements;
  admin delegates to it and the adapter uses the same controller path.
- Convert gauges and cumulative monotonic counters without changing values,
  units, or attributes. Leave counter start times unset; use collection time
  for emitted points. Measurement metadata is not exported.
- Attach the producer to ManualReader and the upstream Prometheus reader.
  Remove MetricsConfig.Meter and RegisterMetricInstruments. The CLI retains
  its startup source check through the ordinary system metrics read.
- The caller owns its pool and custom provider. The convenience exporter
  owns only its private provider; stopping it does not close the pool.

## Consequences

New names appear on the next collection. Reader wiring changes in the same
chunk so the producer replaces the existing adapter rather than living beside
it. Portable family rejection, source-read health, and the full lifecycle
verification checkpoint remain open; HTTP 200 alone still proves no source
health. No new dependencies or compatibility path are added.
