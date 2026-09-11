---
status: accepted
date: 2026-09-11
phase: pre-v1
---

# 0756 — Accept all wears the dark pattern

## Context

[0599] gave the consent bar three equally prominent buttons, on the
reasoning that equal prominence is the compliance-vendor convention. It
is not. The banner a reader has dismissed a thousand times sells one
button and buries the other two: Accept all is large and lit, Reject is
a footnote, Manage preferences is a link. Equal prominence made the bar
politer than the thing it imitates, which blunted both the disguise and
the punchline -- Accept all is the answer that hacks the reader, so the
bar should be seen pushing it.

## Decision

Act one's buttons take the dark pattern the real banners use, and lean
past it.

- **Accept all** keeps the era face, a step larger than its neighbour,
  and gains what the other buttons lack: a gold rim that never stops turning, with a
  pulsing amber glow. The rim is the button's own border-box paint
  layer -- a conic gradient whose angle is a registered custom property
  -- so it animates in place with no wrapper and no z-index step.
- **Reject non-essential** stays the plain era button beside it, and
  **Manage preferences** stays the buried link: the rim alone carries
  the hierarchy.
- The hacked modal plays the same pattern: "please no, I'll do
  anything" -- the link that stars the repo -- is the larger button
  with the rim, and "accept fate" is the plain one beside it.
- The rim is the era button's `data-bait` state in the utilities layer,
  its keyframes beside it, and the tokens layer owns the rim and the two
  glow shadows (`--era-bait-rim`, `--shadow-era-bait`, `-peak`); amber
  is stated once and the night board inherits it, as the hacked veil
  does. Each surface sizes its own bait button, as it already sizes
  its buttons.
- All motion sits behind `prefers-reduced-motion`: a reader who opts
  out gets the same gold rim, still.

Rejected: an oversized Accept all with Reject shrunk to a footnote
(tried, and it read as a parody of a banner instead of a banner -- the
disguise has to hold until the press); a hover-only rim (the point is
that it draws the eye before the pointer arrives).

## Consequences

- [0599]'s equal-prominence bullet is superseded; the rest of that
  record -- the two acts, the privacy statement, the storage key -- is
  unchanged.
- The labels, the accessible names, and which act two each button
  reaches are untouched: the pattern is visual only, and every answer
  still tells the reader the truth.
