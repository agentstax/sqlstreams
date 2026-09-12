---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# Dependabot covers all active Go modules in one group

## Context

Root, OTel, and CLI v0.1.0-rc.1 are published. Dependabot version updates
still cover only the root Go module, while the dev modules depend on the
gitignored workspace and omit a resolvable root requirement. The user
confirmed grouped security updates are enabled and approved expanding
version-update coverage to all seven active Go modules.

## Decision

Pin .bench, .tests, .tools, and examples to the real root v0.1.0-rc.1 and
tidy/check each with GOWORK=off. These modules remain unpublished; normal
development continues through go.work, which selects current local source.
Their standalone dependency graph uses published modules without replaces.

Use one gomod entry with explicit directories for root, cmd/sqlstreams,
otel, .bench, .tests, .tools, and examples. Keep the monthly schedule,
seven-day cooldown, and go-dependencies group matching all dependencies.
GitHub Actions and npm entries keep their existing settings.

Exclude .tools/compat: its intentional prior-release dependency must not
be advanced by routine updates. Explicit directories also avoid enrolling
future modules without checking their standalone dependency resolution.

## Consequences

Dependabot can update the active modules in one Go group while the
workspace continues to test development source. Root pins move only to
real published versions; new imports absent from that version require
a published version before standalone tooling can resolve them.

Local checks prove resolvability and configuration shape. A user push and
hosted Dependabot update run still prove directory coverage and grouping.
Security updates retain their separate repository-level grouping policy.
