---
status: accepted
date: 2026-09-05
phase: pre-v1
---

# 0657 — The datastore holds Logger and Retry once

Amends [0625] and [0636].

## Context

[0625] said the client holds the ambient config once and no resource
config carries `Logger` or `Retry`. The library never got there: the
three declaration configs still end in the pair, the client patches them
in nil-if-unset, and `ConsumerConfig.Retry` sits beside
`ConsumerConfig.Message.Retry`, the trap [0625] named. The client guide
already claims the pair is gone.

Splitting each declaration into "assembler config" plus "declaration"
needs a name for the assembler's config, and `ConsumerConfig` is taken:
after [0653] the bare noun is the resource on the facade, so the
assembler struct would need a role word. `Provisioner` is an interface
with another verb; `Assembler`, `Registrar`, `Factory` are new words for
a struct CONVENTIONS already names by its agent noun. The facade is frozen.

Every constructor already takes `ds`, the client builds the datastore
itself [0636], and `ClientConfig` is `PostgresDatastoreConfig` plus two
flags: `Schema`, `Logger`, `Retry`, then `AllowDestroy`, `DisableManager`.

## Decision

`PostgresDatastoreConfig` gains `Logger` and `Retry`; `PostgresDatastore`
carries the resolved pair and binds the `schema` log attribute where
`Schema` is known. The client passes `ClientConfig`'s values through and
nothing else in the library declares the pair: the three declaration
configs lose their two fields, the assembler structs and constructors
keep their names, and every controller, datastore, worker, and assembler
reads `ds.Logger` and `ds.Retry`. Config structs that held only the pair
are deleted with their constructor param; those with other fields keep
them, including per-loop retry curves. `pkg/metrics/producer`'s
`ProducerConfig` renames to `MetricsProducerConfig`, the config named for
its struct.

## Consequences

The facade is unchanged except for two fields leaving each of
`ConsumerConfig`, `ProducerConfig`, `SchedulerConfig`. Internal callers
that passed the pair per Register call stop. One client is one datastore,
so a second logger means a second client, which was already true.
`pkg/datastore` grows a logger; it is off the doc site since [0637] and
`Retry` was already its mechanism (`DatastoreRetry.Wrap`). A nested
producer or consumer instance logs as itself: its own suppression window
and its own attributes, not its owner's. A constructor takes a logger
only for a suppression window or bound identity `ds.Logger` lacks.

Rejected: a role-word assembler rename with `<Subject><Role>Config`; the
resource renamed back to `ConsumerGroup` (touches the frozen facade);
Register taking the pair as bare params (per-call ambient, seven params).
