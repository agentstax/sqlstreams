---
status: accepted
date: 2026-09-05
phase: pre-v1
---

# 0656 — Search-engine indexing has one path policy

## Context

`/search/` and `/whats-new/` are visitor utilities rather than durable
documentation results. The sitemap already excluded them, but their HTML still
invited indexing, and repeating the route list in both places would let the two
boundaries drift.

Meta keywords do not improve this boundary, and tag archives without distinct
content would add thin result pages rather than useful landing pages.

## Decision

`isSearchEngineIndexable` owns the exact excluded pathnames and normalizes a
trailing slash. The shared layout emits `<meta name="robots" content="noindex">`
when that policy rejects the current route, and the Astro sitemap integration
uses the same policy to filter generated URLs.

Ordinary documentation pages emit no robots override. The site adds neither
meta keywords nor tag archive routes.

## Consequences

The two utility pages remain navigable and crawlable through normal links but
are not search results or sitemap entries. One policy change updates both
boundaries. Unit coverage checks path normalization, while a built-site flow
checks the rendered metadata, generated sitemap, and absence of keyword and
tag surfaces.
