---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# Repository cleanup preserves records and repairs site links

## Context

The cleanup audit found obsolete starter documentation, stale references,
and decision links that resolve as files locally but fail on the website.
The current Git index contains assets and benchmark archives, but no compiled
executables. Old executable blobs remain in reachable Git history.

## Decision

- Keep Git history and root Markdown documents in place, as requested.
- Preserve benchmark evidence and decision bodies; correct ledger references.
- Remove the obsolete Starlight starter README and ignore compatibility
  worktrees at the path documented by the compatibility lab.
- Repair current documentation links and register the existing Maintenance
  page in the reference board so the site emits its route.
- Extend the existing decision Markdown transform to map relative record
  filenames to site routes, preserving fragments and link labels.
- Use a temporary audit of Markdown targets and rendered routes and anchors.
  Add no permanent crawler, dependency, or CI gate.

## Consequences

Decision records keep working as repository Markdown and rendered pages.
Focused transform tests cover relative links and reference definitions.
The audit is a point-in-time result; it does not prevent future link drift.
External services that deny automated requests remain unverified.
