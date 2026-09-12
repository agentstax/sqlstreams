---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# Accept all memes use WebP images

## Context

The Accept all modal loads a 302,300-byte PNG at 180px wide and an
798,178-byte GIF at widths up to 250px. These images load when the modal
opens, rather than during ordinary page reading.

## Decision

Keep the originals and check in WebP derivatives using the existing Sharp
dependency at quality 80. Resize the still to 360 × 234px for its 180px
display. Convert the GIF with `animated: true` at its original 480 × 390px
frame dimensions, preserving all 22 frames, their delays, and looping.

Use the derivatives in the existing meme declarations and update the
still's natural dimensions. Placements and rendered widths stay unchanged.
Replacing an original requires regenerating its derivative; no build step.

## Consequences

The still is 15,762 bytes and the animation is 591,668 bytes. Together
they remove 493,048 bytes from the modal's image downloads, at the cost
of lossy compression. Repeated animation placements use the same URL.
