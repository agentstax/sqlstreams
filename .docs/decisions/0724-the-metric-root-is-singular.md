---
status: accepted
date: 2026-09-09
phase: pre-v1
---

# 0724 -- The metric root is singular

## Context

Every thing-domain root is named for its resource in the singular (topic,
system, worker, alert), and the CLI names each resource the same way
(`vulkan topic`, `vulkan alert`). The metrics domain was the one plural
root: `pkg/metrics`, `MetricsController`, `MetricsTopicName`, and
`vulkan metrics list` sat beside `pkg/alert`, `AlertController`,
`AlertTopicName`, and `vulkan alert list`. Inside the root the vocabulary
was already singular (`MetricKind`, `MetricUnit`, `MetricDefinition`, the
declared `Metric*` vars), so the plural lived only on the machinery and
the CLI.

## Decision

The root is `pkg/metric`, and every declaration built on the root's noun
follows: `MetricController`, `MetricDatastore`, `MetricProducer`,
`MetricCollector` and their configs, instances, and provisioner,
`MetricTopicName`, `MetricCollectorProgressAlert`. The CLI group is
`vulkan metric` (`metric list`, `metric get <name>`).

Collections keep their plural: the `.Metrics()` handles and their
`*MetricsHandle` types pair with `.Alerts()`; the reserved topic stays
`__system.metrics` beside `__system.alerts`; `diagnostic.Metrics()` is a
registry read like `Errors()`. Stored strings are untouched: the worker
name `metrics_collector` and the alert name `metrics_collector_progress`
are rows, and renaming them is a migration, not a rename.

`--metrics-address` stays: it names the Prometheus `/metrics` path, which
is Prometheus's noun. The otel module's `Metrics` adapter and
`MetricsConfig` name the OpenTelemetry metrics pipeline they feed and keep
that noun too.

## Consequences

CONVENTIONS' root list reads `metric`. E2E program directories
(`.e2e/metrics`, `.e2e/metricscollector`) and their recipes keep their
names; they are named for the subject, not the root. Historical records
and HISTORY entries keep the paths they recorded.
