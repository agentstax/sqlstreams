---
status: accepted
date: 2026-09-12
phase: "pre-v1"
---

# CLI registration, health, and worker reads name their client operations

## Context

The CLI verb alignment left stream get calling both Get and Health, and
consumer config get displaying Workers. Consumer Get and Consumers had no
CLI paths. System get lacked the silent existence check used by other gets.

## Decision

- Stream get calls Get only and displays the registered config. Stream health
  calls Health and owns payload-version retirement verdicts.
- Consumer get reads Get; consumer list reads the stream's Consumers.
- Consumer worker list reads Workers and keeps the existing stored-config
  projection and optional key filter. Resource collections use singular noun
  plus list, as stream list and consumer binding list already do.
- Remove consumer config get without an alias. Examples name stored fields;
  ConsumeOptions session settings are not worker config.
- Consumer get and system get support --quiet for silent existence checks;
  consumer list supports --quiet for names. Reject --quiet with --output json.

## Consequences

Stream get JSON drops versions; stream health returns that verdict array.
Pre-v1 scripts reading versions must move to stream health. Scripts using
consumer config get receive exit 2 and must use consumer worker list.
Consumer get JSON is the client row, or null with exit 1 for absence; its
field names follow the client's group and group_id JSON contract.
The command split adds read paths without changing the library or datastore.
