---
status: accepted
date: 2026-09-11
phase: pre-v1
---

# Proposed site pages exist only for features a user consumes

## Context

The doc-page-first rule (the site page is the proposal) was applied to
everything ahead of the library, so developer tooling specs landed on
the site: a Proposed reference page for the test fixture package
(deleted 2026-09-09), two Proposed sections on the reliability-lab page
for the chaos run and the scenario manager, and a demo page for a CLI
command whose premise, the packaged failure-injection e2e tests, was
gone once those tests became integration tests. The rename proposal was
removed for the same reason in [0728]. The site was being used as the
TODO for bench work, and each tooling design change edited a reader page.

## Decision

A Proposed page or section exists only for a feature a user consumes:
a library verb, a config field, a CLI command a reader runs against
their own deployment. Developer tooling (.bench, .tools, .tests, dev
recipes) is specced in ROADMAP/TODO and its decision record and never
appears on the site until it ships and a reader needs it.

The reliability-lab page is deleted: after its two Proposed sections
went, what remained was .bench operator documentation. The chaos-run
shape moves to the Recovery-under-load roadmap item and the manager spec
to TODO. The demo page and its diagram are deleted and the command
becomes a roadmap item that depends on the manager. The metrics-export
and alert-history concept pages, written as the proposal for the otel
and history-alert round and left standing after it shipped, are deleted;
their reader-facing facts move to the metrics and alerts reference
threads and the rest stays in records 0683-0709. The rewind, redrive,
and outcome-reading proposals stay because a user consumes them.

## Consequences

AGENTS.md and CONVENTIONS ## Documentation carry the rule. A tooling
design round produces a roadmap sub-bullet or a record, not a site diff.
The demo command, when picked up, gets its page back as the proposal,
since a reader runs it.
