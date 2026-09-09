---
status: accepted
date: 2026-09-09
phase: pre-v1
---

# SQLStreams is the project name

## Context

The project is renaming away from Vulkan before v1. The README already
uses SQLStreams artwork. Initial research found other SQLStream products;
the user considered their visibility insufficient to change the choice
and explicitly locked in SQLStreams.

## Decision

The product name is SQLStreams, with that capitalization and plural ending.
Do not reopen the naming shortlist because of the identified SQLStream
products. The resource currently called a topic becomes a stream across
the library, CLI, storage vocabulary, observability and website.

Message, producer, consumer, consumer group, binding, cursor, lease,
schedule and system keep their meanings. The new name supplies no new
delivery or ordering guarantee.

## Consequences

The rename includes the website and a new logo sheet. Module, binary,
prefix and docs-origin choices follow from this identity; their exact
forms and database cutover remain implementation-planning work.
Current code and shipped documentation still use Vulkan until the
documentation-first proposal and implementation land.
