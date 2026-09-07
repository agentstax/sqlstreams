---
status: superseded
date: 2026-09-07
phase: "pre-v1"
---

# Portable export validation and read health

## Context

Superseded by [0707], which narrows validation to export compatibility.

The SDK producer replacement [0705] is complete. The approved contract [0682]
requires visible read failure and collection-local family rejection without
preventing healthy families from exporting.

## Decision

- Validate retained families before conversion using the pinned upstream
  UnderscoreEscapingWithSuffixes metric and attribute translation. Configure
  the convenience Prometheus reader with the same strategy.
- Reject the whole original-name family for conflicting kind/unit, ambiguous
  translated metric or attribute names, reserved names, or unusable kind/unit.
  Built-ins must match their declaration. Reserve exporter health names even
  against retained built-ins, Vulkan's custom namespace, target_info, and
  otel_scope_ metadata names; reserve double-underscore attribute names too.
- Return source.read_success (VK0102) as 1 on successful reads, including empty
  results. Return measurements.rejected (VK0103), counting each omitted retained
  row once. Both are attribute-free collection-local gauges, never persisted.
- On read failure, return read-success 0 together with the source error; omit
  retained measurements and the rejection count. Partial rejection returns no
  collection error, so healthy families remain eligible for periodic export.
- Log VK0104 per rejected family with original metric_names and detail, never
  measurement values or metadata. Reevaluate every collection without a cache.

## Consequences

ManualReader retains read-success 0 alongside the error, and Prometheus can
serve it with HTTP 200. PeriodicReader skips export on collection errors;
its full lifecycle verification remains a separate checkpoint. Portable
validation also applies to non-Prometheus readers. Translation dependencies
stay pinned; no new version or module is introduced.
