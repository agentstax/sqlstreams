---
status: accepted
date: 2026-09-05
phase: pre-v1
---

# The migration version reads through the client

## Context

`vulkan migrate status` and the six `migrate <scope> up|down` leaves read
each target's current schema version before any DDL runs: the direction
guard, the no-op check, and the status table all need it. No client verb
exposed that read, so the CLI reached under the client with
`client.Datastore()`, built its own `migratecontroller.Controller`, and
composed `common.NewTopicOwner` from `GetTopic` rows twice to feed
`TopicVersion`. [0649] had just moved owner resolution into admin's
`SystemOwner` / `TopicOwner` / `GroupOwner`; the CLI copies were the last
callers assembling ids by hand outside the library.

## Decision

- The version a scope's tables are at is a read on the scope's handle:
  `client.System().MigrationVersion(ctx)` and
  `client.Topic[T](name).MigrationVersion(ctx)`, both `(int64, error)`.
  The name is the `migration_log.migration_version` column's, and it
  keeps its distance from `Versioned.SchemaVersion`, which versions
  payloads.
- admin composes each read the way it composes `MigrateSystem` /
  `MigrateTopic`: `SystemMigrationVersion` resolves `SystemOwner` then
  calls the migrate controller's `SystemVersion`; `TopicMigrationVersion`
  resolves `TopicOwner` then `TopicVersion`. Absence surfaces as the
  existing errors, `ErrNotRegistered` and `ErrTopicNotFound`.
- The read returns the bare version. The datastore's `SchemaStateRow` also
  carries the minimum compatible version in force, but nothing on the
  client or CLI reads it, so it stays internal until a consumer appears.
- The CLI's `migrateTarget` holds a name and a version, no owner, and
  `gatherTargets` takes only the client. `migrate status` builds no
  controller at all. The advisory-lock pre-flight (`IsLocked`) still builds
  one from `client.Datastore()`, as `destroy` and `manager run` do for their
  own reads; exposing the lock probe on the client is a separate decision.

## Consequences

- No caller outside the library composes an owner from catalog rows; the
  three admin resolvers are the only path, and a destroyed topic reads as
  not-found rather than as a stale id.
- `migrate topics` and `migrate status` resolve each topic by name, one
  catalog read per topic beside the version read. The lists are operator
  sized, so the extra round trip is accepted over a second id-keyed read
  path.
- The client guide's sample and old-verbs table and the migrations guide
  name the two reads.
