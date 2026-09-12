---
status: superseded
date: 2026-09-12
phase: pre-v1
---

# 0769 — The profile fades while idle and its personal text grows

Superseded by [0772](0772-profile-scroll-slows-with-text-growth.md), which
adds slower scrolling as size increases and retains the fade and lifecycle.

## Context

The member profile's repeating "the end is never" should gradually take
over the page. The user reviewed a timing sketch, chose inactivity as the
trigger, and approved implementation with a five-second initial delay.
Movement, keyboard input, and touch must bring the profile back.

## Decision

- After five idle seconds, fade the surrounding page into its background
  over 110 seconds. Scale the personal text from 13px to an apparent 113px
  over the same interval, using quadratic growth so it becomes more
  noticeable later. Hold that size and continue scrolling until activity.
- Keep the effect in the route-local member-personal-text component and
  its runes state class. Hydrate MemberProfile with client:idle. A fixed,
  pointer-transparent veil covers the viewport; the personal text rises
  above it only while the effect is active. Two semantic z-index tokens
  add these steps to the existing scale; BoardLayout needs no changes.
- Scale a wrapper around the existing two-copy scrolling loop, preserving
  its phase. The enlarged line overlaps the fading profile rather than
  reflowing it, so links stay under the pointer when activity restores them.
- Pointer movement/presses, keyboard input, wheel scrolling, touch movement,
  and focus restore immediately and restart the five-second delay. Layout
  scroll events are not reader activity. No event is intercepted or canceled.
- Reduced motion disables the effect and the existing scroll animation.
  Hiding the document resets it; returning starts a fresh delay. Abort
  listeners and cancel pending animation/timers before an Astro page swap
  and on component teardown. Returning to the profile creates fresh state.

## Consequences

- The rest of the site has no idle listeners or changed layout. The
  profile gains a small hydrated island without a dependency.
- Both board styles use their existing page background color. The veil
  also fades page notices; any interaction restores their normal layering.
- Browser flows cover the delay, growth, stable link positions, input
  restoration, reduced motion, hidden time, and navigation away and back.
- Television noise is not part of this treatment. The approved sketch is
  removed once the implementation and verification are complete.
