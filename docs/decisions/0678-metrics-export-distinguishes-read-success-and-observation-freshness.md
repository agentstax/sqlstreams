---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Metrics export distinguishes read success and observation freshness

## Context

The otelvulkan review found that retained values lose Measurement.At at
export, source-read failures can yield HTTP 200, and discovery and callback
lifetime are incomplete. The stored-measurement model [0522] also serves the
core history API. River records activity directly into OTel; RabbitMQ
separates statistics updates from scrapes. Neither comparison requires
Vulkan to discard retained history.

## Decision

- Keep retained measurements as the export source. Collection, production,
  and history stay in core; OTel conversion and Prometheus exposure stay in
  the optional module. Delegate measurement lookup to core's existing path.
- A source-read failure must fail the Prometheus scrape with HTTP 5xx;
  caller-owned OTel collection receives the read error through its pipeline.
  Successful source reads and observation freshness are separate facts.
- Expose freshness without treating every old measurement as a stopped
  producer. Collector gauges refresh periodically, session counters skip
  unchanged totals, and custom measurements have application-owned cadence.
  A partially completed collection also prevents one recent series from
  proving that the entire deployment was measured successfully.
- Both integrations own discovery of new metric names during operation.
  Stopping an adapter detaches its callbacks and discovery work; a caller's
  provider and pool remain caller-owned. The exporter closes its own provider.
- The doc-site proposal is concepts/metrics-export. Freshness representation,
  lifecycle signatures and cadence, unexportable custom measurement handling,
  and deployment scope remain open. This record settles direction only;
  implementation follows proposal review.

## Consequences

Retained history remains available without external monitoring. Scrape
frequency does not force deployment measurements to run more often. The
current implementation does not yet satisfy this contract; the page is
marked Proposed. A universal age cutoff and a change to direct OTel
instrumentation are not part of this work.
