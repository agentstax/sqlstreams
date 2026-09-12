---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# Header wordmark links home

## Context

The shared header displays the SQLStreams wordmark as a plain image.
Readers expect clicking it to return to the home page.

## Decision

Wrap the wordmark in a native link to `/`, named “SQLStreams home”.
Keep the link fitted to the image and use the existing focus indicator.

## Consequences

The wordmark supports pointer and keyboard navigation on every shared-layout
page without JavaScript. Frozen deployments link to their own home page.
