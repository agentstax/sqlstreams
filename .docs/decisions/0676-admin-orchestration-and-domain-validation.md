---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# 0676 — Admin owns orchestration and domains own validation rules and resource reads

## Context

The admin review found duplicated validation, topic validation after automatic
bootstrap, consumer worker selection repaired by filtering an owner-chain
read, and a public health type declared in an API package. The user accepted
the concrete implementation shapes below; implementation remains pending.

## Decision

Admin keeps assembly, cross-domain identity resolution, operation policy,
and delegation, including bootstrap, migrations, and composed destroy guards.
Controllers own domain verbs and persistence invariants; datastores own SQL.
Admin may validate before orchestration. Repeat simple name checks explicitly
and call existing config validation; do not add a name type or shared helper
solely to remove that duplication. Validate topic input before bootstrap;
retain controller validation for direct callers and automatic bootstrap [0624].

Add ListConsumerGroupWorkers to the worker controller and datastore. Admin
resolves the owner and delegates; the query selects consumer_group_id directly.
Reuse existing row/adaptation behavior. Keep the manager's owner-chain read
unchanged. No scope enum or generic selection API is introduced.

Move TopicVersionHealth's declaration to pkg/topic and retarget the vulkan
alias. Inline the existing verdict logic in admin's TopicHealth loop, keeping
compaction-head precedence and all fields, JSON tags, and reason strings.
Supersedes [0411](0411-familyhealth-lives-in-pkg-admin.md) on declaration
placement; its single metrics read path and admin computation remain intact.

## Consequences

Invalid topic input no longer creates system resources before returning an
error. Later write failures can still leave partial registration progress.
Worker selection and health retain their results and diagnostic behavior.
Explicit small duplication is preferred to additional validation or selection
abstractions. Binding conventions and public-behavior proposals must be aligned
before implementation; docs/TODO.md carries the execution and verification plan.
