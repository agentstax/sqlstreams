---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# Avatar images match their display sizes

## Context

Posts display the cat avatar at 44px and the member profile at 88px.
Both downloaded the same 94,408-byte PNG, whose source is 549 × 449px.

## Decision

Keep `public/cat.png` as the original. Derive `cat-88.webp` for posts and
`cat-176.webp` for the member profile using the existing Sharp dependency:
`resize({width}).webp({quality:80})`. Preserve transparency and source
proportions in the files, and retain the existing rendered dimensions.

Check in both derivatives; replacing the original requires regenerating them.
No new build step or image component is needed for these two fixed uses.

## Consequences

The files are 1,968 and 4,346 bytes, respectively. Each supplies twice the
displayed width, with lossy compression and no additional variants for 3×
screens. The two pages use distinct cache entries; repeated posts share one.
