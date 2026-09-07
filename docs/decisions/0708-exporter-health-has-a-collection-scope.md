---
status: accepted
date: 2026-09-07
phase: "pre-v1"
---

# Exporter health has a collection scope

## Context

The collection-local health gauges from [0707] were declared system-scoped.
That made the stored-resource catalog promise two metrics that have no stored
values or resource selectors. The selector coverage test exposed the mismatch.

## Decision

- Add exporter to the existing MetricScope vocabulary. Declare VK0102 and
  VK0103 in that scope; keep their names, units, and export behavior unchanged.
- The whole metric catalog includes exporter health. Resource-scoped catalogs
  exclude it; do not invent Latest/History selectors for collection-local data.
- Alert declarations continue to require a resource scope. Reject exporter
  scope just as consumer-session scope is rejected.

## Consequences

Catalog callers can distinguish stored resource metrics from export health
without a second registry. The generated site catalog carries the same scope.
No schema migration or measurement production change is needed.
