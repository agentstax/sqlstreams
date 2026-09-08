---
status: accepted
date: 2026-09-05
phase: pre-v1
---

# 0652 — Astro's build emits the canonical sitemap

## Context

Every page already rendered static HTML with a canonical URL, but the deployed
site exposed no sitemap and its Cloudflare-generated `robots.txt` advertised
none. The visitor-specific search and what's-new pages are useful navigation,
not stable search-result destinations. Hand-writing the site's 500-plus URLs
would create a second route registry beside Astro.

## Decision

Astro's official sitemap integration emits the sitemap from the built routes
and the configured `siteUrl`. Its filter omits `/search/` and `/whats-new/`;
Astro omits its 404 route. A statically rendered `robots.txt` route derives the
sitemap index URL from the same configured site and otherwise says only
`Allow: /`.
Cloudflare may prepend its managed content signals to that origin response.

No `lastmod`, `changefreq`, or `priority` values are invented. Accurate
per-page dates can be added only if one existing source supplies them to the
sitemap build.

## Consequences

Adding or removing a static page updates the sitemap without another registry.
Frozen deployments still name the live origin, matching their canonical tags.
Google and Bing submission remains an operator step after deployment, tracked
separately in the roadmap.
