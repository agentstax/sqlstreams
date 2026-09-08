---
status: accepted
date: 2026-09-05
phase: pre-v1
---

# 0654 — The shared layout qualifies document titles

## Context

Every route gives `BoardLayout` the page title that also feeds a visible page
component. Rendering that value unchanged in HTML's `title` leaves search
results and browser history without a consistent site identity. Changing the
route title itself would also change visible headings and breadcrumb labels.

## Decision

`BoardLayout` derives the HTML document title as
`<page title> | Vulkan Docs`. The incoming `title` prop stays unchanged and
continues to feed visible page components through their existing callers.
Because every rendered HTML page uses this layout, the rule applies once to
threads, code pages, board indexes, utility pages, and the 404 page.

## Consequences

Search results, tabs, and browser history carry the Vulkan Docs identity while
visible H1 text remains the authored page title. Canonical URLs, content
frontmatter, Pagefind titles, breadcrumbs, and social content are unchanged.
A browser flow asserts both the qualified document title and the unchanged H1.
