---
status: superseded
date: 2026-09-12
phase: pre-v1
---

# 0773 — Profile scroll slowdown is eased slightly

Superseded by [0775](0775-profile-idle-delay-three-seconds.md), which
shortens the idle delay and retains this scrolling-speed curve.

## Context

After trying the growing text, the user requested slightly more scrolling.
This supersedes [0772](0772-profile-scroll-slows-with-text-growth.md) for
the speed curve and retains its animation, fade, and restoration decisions.

## Decision

Use `1 / scale^1.9` instead of `1 / scale²` for playback rate. Scrolling
still slows progressively, but at maximum size its pixel speed is about
14.3% of the initial speed rather than 11.5% — approximately 24% faster
than the previous treatment. Initial speed remains the same.

## Consequences

Only the exponent changes. The existing browser invariant remains valid:
scrolling slows as text grows, stays positive, and resets on activity.
