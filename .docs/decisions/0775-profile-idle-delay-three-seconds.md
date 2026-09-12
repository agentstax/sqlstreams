---
status: superseded
date: 2026-09-12
phase: pre-v1
---

# 0775 — The profile effect begins after three idle seconds

Superseded by [0776](0776-profile-text-expands-to-viewport-width.md), which
adds viewport-width expansion and retains the three-second delay.

## Context

The user requested a shorter wait before the profile effect starts.
This supersedes [0773](0773-profile-scroll-slowdown-eased.md), retaining
its scrolling speed and the existing fade, growth, and restoration behavior.

## Decision

Start the effect after three idle seconds instead of five. Every activity
reset and return from a hidden tab uses the same three-second delay.

## Consequences

Only the delay constant changes. The initial-delay and hidden-tab tests
move their pre-start assertions from 4,999ms to 2,999ms to pin the requested
timing; the remaining behavior assertions stay unchanged.
