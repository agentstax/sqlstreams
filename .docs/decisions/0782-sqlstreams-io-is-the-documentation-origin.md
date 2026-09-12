---
status: superseded
date: 2026-09-12
phase: pre-v1
---

# sqlstreams.io is the documentation origin

## Context

Retaining the old Pages project and branch aliases is superseded by [0784].
The canonical documentation origin decision remains in force.

The user purchased sqlstreams.io through Cloudflare and attached it to
the existing vulkan Pages project. HTTPS serves the docs; HTTP and HTTPS
www requests redirect to the root with paths and query strings preserved.
This supersedes the temporary-origin portion of [0726]; its disposable
database cutover decision remains in force.

## Decision

Use https://sqlstreams.io as the website's canonical origin and the main
version's URL. Diagnostic links, CLI output, README links, and package
manager homepage and docs metadata use the same origin.

Keep the existing Cloudflare Pages project and its frozen-version branch
aliases. Historical records retain the deployment URLs they described.
Cloudflare owns the www redirects; the repository needs no redirect code.

## Consequences

The site build emits canonical links, sitemap URLs, robots.txt, and the
version-manifest fetch URL under sqlstreams.io. Newly built library and CLI
diagnostics link to sqlstreams.io/errors/<code>.

Publishing the site build still requires deployment approval. Future
binary releases embed the permanent domain without another URL cutover.
