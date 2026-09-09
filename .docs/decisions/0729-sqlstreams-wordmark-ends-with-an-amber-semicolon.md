---
status: accepted
date: 2026-09-09
phase: pre-v1
---

# SQLStreams wordmark ends with an amber semicolon

## Context

The user approved the local logo sheet after correcting semicolon
placement and the website's SQL color. The existing README lettering
already provides the SQLStreams identity; the website still used Vulkan's
pixel volcano.

## Decision

Use the existing outlined SQLStreams lettering with a trailing semicolon:
SQLStreams;. Never place the semicolon before the name. SQL and the
semicolon use amber; Streams uses dark ink on light backgrounds and light
ink on dark backgrounds. The website's blue header uses the dark treatment.

Use the standalone semicolon for the favicon and avatar. The stream-S and
message-log alternatives are not selected. Keep the website's board layout
and palette; replace its volcano/banner wordmark with the approved artwork.

Deliver SVG masters and PNG exports for wordmarks, favicon, avatar and
social sharing. The SVGs preserve outlined letters and require no font at
render time. Accessible labels spell SQLStreams without punctuation.

## Consequences

The lettering comes from the repository's existing sqlstreams SVGs; the
new symbol geometry comes from the local logo sheet. No external font or
artwork is added. The original source assets do not identify their typeface;
retain that provenance limit rather than inventing a font attribution.

The logo sheet remains a local review artifact until task close-out.
