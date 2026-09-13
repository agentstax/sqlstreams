---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# Release maintenance, deprecation, and private security reports

## Context

Downstream users need to know which releases receive fixes, how much notice
API removals receive, and where to report vulnerabilities privately. The
PostgreSQL 15–18 investigation passed existing integration tests, but further
verification remains in Next. Both migration registries still have no steps.

## Decision

Maintain only the latest stable SQLStreams release, including stable v0.x
releases. Deliver bug and security fixes in a new release; promise no
backports to older release lines.

Before removing or incompatibly changing a supported public API, announce
the deprecation in release notes, mark affected Go declarations Deprecated:,
and document the replacement and migration steps. Retain working APIs through
at least one subsequent minor release. An urgent security exception explains
its reason and migration steps in the release notes.

Use GitHub private vulnerability reporting, linked from .github/SECURITY.md.
Aim to acknowledge reports within seven calendar days, without promising a
resolution deadline. The repository setting was verified enabled on this date.

When the next real schema change ships, demonstrate the declared old/new
binary compatibility, preservation of messages and consumer progress, and
recovery after an interrupted migration. Extend existing verification recipes
only as needed; do not manufacture a migration for evidence.

## Consequences

The upgrade guide and contributor rules carry the maintenance and deprecation
policy. The security policy gives reporters an existing private route.
This policy applies going forward; it does not establish compatibility for
earlier pre-v1 baseline edits or complete the PostgreSQL support matrix.

Further PostgreSQL verification stays in Next. Evidence for a real schema
change stays in Later until that change is selected. Existing release
checkpoints remain required; these policies do not claim unperformed tests.
