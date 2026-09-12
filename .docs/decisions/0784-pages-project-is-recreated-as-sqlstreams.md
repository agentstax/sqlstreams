---
status: accepted
date: 2026-09-12
phase: pre-v1
---

# Pages project is recreated as sqlstreams

## Context

The user wants the generated pages.dev address to match SQLStreams, has
deleted the vulkan Pages project, and accepts downtime during replacement.
Renaming a Pages project does not change its generated subdomain.
This supersedes [0782] only on retaining the old project and branch aliases;
sqlstreams.io remains the canonical documentation origin.

## Decision

Create the sqlstreams Pages project with main as its production branch,
deploy the prepared site, and attach sqlstreams.io as its custom domain.
Point the root's proxied CNAME at the new project's assigned pages.dev
address. Keep the existing www-to-root redirects and www DNS record.

Both deployment recipes target sqlstreams. Future frozen versions use
the replacement project's branch aliases. Historical records retain the
old URLs; the current version manifest contains only main at sqlstreams.io.

## Consequences

The old project's deployment history and preview URLs are gone. The site
and DNS checks must pass on the replacement before the move is complete.
The user authorized recreation and deployment after accepting downtime.
