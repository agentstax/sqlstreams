---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# 0778 — The profile effect begins after four idle seconds

## Context

The user requested four idle seconds before the effect begins.
This supersedes [0776](0776-profile-text-expands-to-viewport-width.md),
retaining its width expansion, fade, growth, scrolling, and restoration.

## Decision

Set the shared idle delay to four seconds. Activity and returning from
a hidden tab restart the same four-second wait.

## Consequences

Only the delay constant changes. Initial-delay and hidden-tab tests move
their pre-start assertions from 2,999ms to 3,999ms to pin the new timing.
