---
status: superseded
date: 2026-09-12
phase: pre-v1
---

# 0772 — Profile scrolling slows as the personal text grows

Superseded by [0773](0773-profile-scroll-slowdown-eased.md), which eases the
slowdown while retaining the same animation and restoration behavior.

## Context

The user approved the existing fade after watching it develop and asked
for scrolling to slow as the letters grow. Scaling a moving line also
multiplies its pixel speed, so reducing the playback rate must account for
both effects. This supersedes [0769](0769-profile-idle-fade-and-text-growth.md),
retaining its fade, growth, inactivity, restoration, and lifecycle decisions.

## Decision

- Keep the five-second delay, 110-second growth/fade, and board-colored
  background. The proposed white fade was withdrawn before implementation.
- Set the existing CSS loops' playback rate to `1 / scale²`. One factor
  compensates for enlargement; the other makes scrolling slower as size
  increases. At the largest size, scrolling contributes about 11.5% of its
  original pixel speed and continues indefinitely.
- Use the browser's `Animation.updatePlaybackRate` on both copies, which
  synchronizes playback position before changing speed. Keep the existing
  CSS loop rather than changing its duration or restarting it. The
  [Web Animations specification](https://drafts.csswg.org/web-animations-1/#dom-animation-updateplaybackrate)
  defines the synchronization behavior.
- Set progress and playback speed together in the state class. Activity,
  hiding the page, or reduced motion restores progress and normal speed;
  the existing cleanup remains responsible for navigation and teardown.

## Consequences

- Growth and slowdown follow the same scale, with no second timer or
  animation implementation. Both repeated copies keep the same rate.
- The browser flow checks that scrolling speed decreases as scale grows,
  stays positive, and returns to normal on activity.
