---
status: accepted
date: 2026-09-08
phase: pre-v1
---

# 0719 -- Programs under .e2e are e2e tests

## Context

The verification programs moved to `.e2e/` but kept `lab` in directory names,
Justfile recipes, source identifiers, output, and prose. That left two names
for the same kind of test and made the new root cosmetic.

## Decision

Every verification program under `.e2e/` is called an e2e test. Directory
names drop the `lab` suffix, Justfile recipes end in `-e2e`, and source code
uses `test` for local helpers and fixtures. The compatibility and reliability
systems keep their established `*-lab` commands because they live outside
`.e2e/` and serve different workflows.

## Consequences

`just reclaim-lab` becomes `just reclaim-e2e`, and
`.e2e/reclaimlab` becomes `.e2e/reclaim`. Current docs and output use the same
terminology. Historical decision records and HISTORY entries retain the names
they recorded.
