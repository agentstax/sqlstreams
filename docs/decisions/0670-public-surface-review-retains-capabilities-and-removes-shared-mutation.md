---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Public surface review retains capabilities and removes shared mutation

## Context

The review under [0665] examined the current client and every reachable alias.
The old package-demotion plan no longer matched the one-client API. Useful
controls needed explicit contracts; shared mutable construction state and
database-specific helpers did not belong on the supported client surface.

## Decision

- Keep [0665]'s boundary: vulkan and reachable exports are supported; other
  packages stay importable advanced options. No internal-package moves.
- Remove the empty nested system config. SystemConfig belongs to system and
  carries alert/collector declarations. ScheduleRunOptions belongs to scheduler;
  RunSchedule stays verb-first. Vulkan aliases preserve declaration names.
- Name registered consumer operations Consumer in admin and CLI; shared
  identity/state uses ConsumerGroup. Apply this to errors, summaries, bindings,
  flags, and JSON. Diagnostic codes and log attribute keys remain unchanged.
- Put metric production/consumption on System().Metrics() handles, retaining
  routing, series compaction, reserved-name validation, and normal upkeep.
  OTel retains Metrics/Exporter integration over caller-owned pools and configs.
- Remove Client.Datastore and its PostgresDatastore alias. CLI owns its pool
  and settings; advanced callers construct datastores over their own pools.
- Remove Client.Config/Logger. Capture settings and copy Retry at construction;
  retain the supplied logger. Preserve schema attribution in CLI manager logs.
- Keep batching, Tx/Querier/Raw, transaction methods, distinct migration and
  destruction scopes, configuration defaults/validation, and operator read-models.
- Keep RetryPolicy calculations/equality and MessageOptions resolution/equality.
  Document copying and nil semantics. Validate rejects total retry sleep overflow;
  producer construction rejects combined sleep/operation-budget overflow.
  CalculateDelay caps before conversion, retaining floating point below the cap.
- Keep Owner fields/Kind; delete unused IdColumns and move nullable owner-column
  conversion to datastore.NewOwnerColumns, outside the supported alias closure.
- Make diagnostic error/event fields private with read accessors. Constructors
  copy queries before registration; Queries returns detached values. Remove
  Diagnose; retain With/Wrap, matching, recovery, rendering, and documentation.
- Keep Worker.Metadata as stored JSON-compatible inspection data, with sparse,
  worker-specific fields outside the stable client contract; local edits do not
  update workers and the document is not an instance's effective configuration.

## Consequences

Removed fields/methods and renamed Go/CLI/JSON names are source-contract breaks;
no compatibility aliases preserve them. Diagnostic export and rendered behavior
remain unchanged. Arbitrary values attached to errors are not deep-copied, and
exported error variables remain assignable. Oversized retry budgets now fail
validation/construction. ScheduledAt separation remains a Now roadmap task;
entry-package relocation and the broader public documentation audit remain
separate work. This single record closes the review and replaces its inventory.
