---
status: accepted
date: 2026-09-09
phase: pre-v1
---

# SQLStreams technical identity follows the product name

## Context

The user approved the website's SQLStreams rename proposal after settling
the product name [0725] and disposable-database cutover [0726]. The
technical names must agree across installation, use and diagnosis.

## Decision

The repository/module identity is github.com/agentstax/sqlstreams. The
public Go package and CLI binary are sqlstreams; the entry package's
location remains the separately scoped relocation task.

Environment variables use SQLSTREAMS_, including
SQLSTREAMS_ADMIN_DATABASE_URL and SQLSTREAMS_ADMIN_SCHEMA. The default
Postgres schema is sqlstreams; explicit schemas remain caller-selected.

Diagnostic codes use SS plus the existing four-digit serial, preserving
the serial across errors, events, metrics and alerts. Metrics use the
sqlstreams. prefix, and the module-version log attribute is sqlstreams_version.

Topic-derived API, CLI, SQL and diagnostic vocabulary becomes stream.
Neutral table-family names and __system.metrics/alerts/schedules remain.
The current delivery, ordering and routing semantics remain unchanged.

## Consequences

The proposal-publication consequence below is superseded by [0728];
the approved technical identity remains accepted.

The public proposal is approved for implementation, but stays labeled
Proposed on the website until shipped behavior matches it. Logo-sheet
review is the next plan step. External repository and release destinations
still need verification before publication; the approved spelling does
not assert that external accounts have already been changed.
