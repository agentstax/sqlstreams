---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# Compatibility checkpoints use the published prior client on current tables

## Context

The v0.1.0 release checkpoint has a published SQLStreams predecessor:
v0.1.0-rc.1. The dormant compatibility driver imports the older Vulkan API
and replaces that module with the renamed working tree. It also creates
its own stream, so its stream lifecycle would test tables from the old
build rather than tables created by the release candidate.

## Decision

Pin .tools/compat to published SQLStreams v0.1.0-rc.1 without replacements.
The just recipe explicitly disables go.work. Adapt the existing driver to
that version's supported client API; retain the actual prior dependency.

The current build prepares both system and stream tables first, using the
existing systemregister and producer examples. The pinned driver requires
both migration versions before registration. It then tests the declared
producer-registration refusal or five distinct produced/consumed payloads
and stream destruction. Missing preparation and an incorrect expected
verdict must fail.

## Consequences

Both registries remain at schema v1 with no migration steps, and library
source is unchanged since the RC. The expected verdict is round-trip.
This checkpoint verifies that exact pair; it does not establish a blanket
compatibility promise for pre-v1 baseline changes or publish v0.1.0.

Future checkpoints advance the real prior-version pin, adapt the driver to
its API, and prepare/migrate both scopes with the current build before use.
