---
status: accepted
date: 2026-09-09
phase: pre-v1
---

# Diagnostic codes use the SQL prefix

## Context

The user requested replacing SS because of its historical association and
approved SQL as the diagnostic prefix. Nothing is public or official yet.
This supersedes only the diagnostic-prefix choice in [0727].

## Decision

Use SQL followed by exactly four ASCII digits for errors, events, metrics
and alerts: SQL0005. Preserve every existing numeric serial. Update code
validation, declarations, CLI explain, log examples, website pages and
exported diagnostic data together. Do not add old-prefix aliases or redirects.

## Consequences

Consumers of pre-release diagnostic strings use SQL instead of SS. Module,
binary, environment, schema and metric-name identities stay as approved in
[0727]. Existing historical records retain the prefix used at the time.
