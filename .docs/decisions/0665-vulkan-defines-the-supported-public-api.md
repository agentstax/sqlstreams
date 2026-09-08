---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Vulkan defines the supported public API

## Context

The three-audience trim [0507] predates the one-client API. Its pending
package demotions would prevent advanced imports that should remain possible.
The entry package will eventually move from pkg/vulkan, but no destination
or module split has been selected.

## Decision

The supported surface is pkg/vulkan's exported names and the exported fields
and methods reachable through its types, aliases, parameters, and results.
Other packages remain importable advanced options, without a stability
commitment or guides presenting alternative public entry points. Comments
for aliased public declarations stay with their owning declarations.

This supersedes [0507]. Do not hide packages or unexport low-level constructors
solely to shrink the supported API. Audit current exposure, including alias
methods, before changing signatures. Existing alias-closure checks establish
name completeness, not whether each exposure belongs.

The current inventory replaces _public-surface.md's obsolete audience lists.
Keep/question/remove verdicts there are recommendations for review, not
accepted removals. In particular, RegisterSystemConfig has real settings;
only its empty SystemConfig member is a removal candidate.

## Consequences

Normal users have one documented entry point. Advanced imports remain
possible, and lower-level declarations exposed through aliases still carry
the supported contract. Third-party types keep their upstream contracts.
The future entry-package move must update imports and path-aware checks;
it does not require moving implementation packages to internal.
