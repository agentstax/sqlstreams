# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

## Doc-site sitemap

- Add Astro's official sitemap integration against the canonical `siteUrl`.
- Publish a `robots.txt` that allows search crawling and advertises the sitemap
  while preserving Cloudflare's managed content-signals prefix.
- Exclude visitor-specific utility routes from the sitemap in the same place
  their `noindex` boundary will be implemented.
- Build and verify the sitemap index, sitemap URLs, and `robots.txt`.
- Record the decision and close the roadmap item into HISTORY.md.
