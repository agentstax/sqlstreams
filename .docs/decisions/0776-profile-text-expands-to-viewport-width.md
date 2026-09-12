---
status: superseded
date: 2026-09-12
phase: pre-v1
---

# 0776 — The profile text container expands to viewport width

Superseded by [0778](0778-profile-idle-delay-four-seconds.md), which sets
the idle delay to four seconds and retains viewport-width expansion.

## Context

The user wants the growing personal text to reach both screen edges as
the surrounding page fades. This supersedes
[0775](0775-profile-idle-delay-three-seconds.md), retaining its delay,
growth, scrolling speed, fade, and restoration behavior.

## Decision

- Expand the existing absolute text container outward in proportion to
  fade progress. It begins at the profile's text-strip width and ends
  at the viewport's left and right edges without moving surrounding links.
- Measure the stationary text window's left and right insets with a
  ResizeObserver. Observe that window and the document root so resizing
  updates the destination while the effect is running. Disconnect on the
  existing teardown path.
- Use documentElement.clientWidth for the right edge, excluding the
  vertical scrollbar. CSS interpolates the two measured insets with the
  existing progress; no second animation or timer is introduced.
- Activity restores the original width with the rest of the profile.
  Reduced motion keeps the normal text strip.

## Consequences

The line reaches the screen edges on desktop and phone without horizontal
overflow. A browser flow checks intermediate width, both final edges,
resizing during the effect, and restoration to the original container.
