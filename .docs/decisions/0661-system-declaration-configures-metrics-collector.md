---
status: accepted
date: 2026-09-05
phase: "pre-v1"
---

# The system declaration configures the metrics collector

## Context

The metrics collector reads its poll rate from its worker row, but the
provisioner declared a fixed 30-second rate. The public system declaration
could configure the three alerts [0658] but could not configure collection.

## Decision

`RegisterSystemConfig.MetricsCollector` takes a
`metrics.MetricsCollectorWorkerConfig` with `PollRate`. Zero defaults to
30 seconds; negative durations are rejected before registration writes.

Each `RegisterSystem` call constructs a collector provisioner with that
rate in its own config and calls its existing `Declare` method. The
provisioner's definition gets its metadata from that config, following
the alert provisioners' config-to-metadata pattern. There is no separate
definition factory or direct collector-row declaration in admin.

## Consequences

Repeated registration updates the same worker row through the existing
declaration path. The manager's provisioner reads the stored rate when
claiming; a running collector retains its rate until its next claim.
Existing worker-config replacement logs report the metadata change, and
the collector's start log reports its effective rate. No migration is needed.
