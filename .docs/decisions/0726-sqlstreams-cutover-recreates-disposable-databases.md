---
status: superseded
date: 2026-09-09
phase: pre-v1
---

# SQLStreams cutover recreates disposable databases and keeps a temporary docs origin

## Context

The temporary documentation origin is superseded by [0782]. The disposable
database cutover decision remains in force.

The SQLStreams rename [0725] reaches database catalogs, stored identities
and diagnostic URLs. The user confirmed that all existing databases are
disposable and that no binaries will be released before a domain is bought.

## Decision

Apply the rename to baseline CREATE TABLE DDL under the existing pre-v1
policy. Stop old processes and recreate disposable databases for the
renamed build. No data-preserving rename migration, old-schema adapter,
or mixed-version deployment support is required for this cutover.

Keep the generated Cloudflare docs address during development. The current
origin can remain; a new generated address is permitted if needed. No
Cloudflare project recreation is required solely for branding.

The permanent domain remains unselected; sqlstreams.io is a candidate.
Before releasing binaries, select the domain and update the website's
canonical origin, version-manifest references and embedded diagnostic URL.

## Consequences

Existing messages, configuration, cursors and history need not survive
the development database reset. Baseline recreation and fresh-DB e2e
verification are part of implementation, not actions performed by this
decision record. The separate benchmark session's live database work must
be coordinated before a shared database is reset.

The rename can proceed without buying a domain. Website deployment still
follows the repository's approval rule. Exact technical prefixes remain
part of the public proposal.
