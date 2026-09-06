---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# The scheduled time is a message_log column on every message

## Context

`MessageOptions.ScheduledAt` was the schedule producer's due time, stored
inside the row's `options` JSON. `MessageOptions` is what a message
requests and a consumer clamps, so the field also appeared in consumer
defaults and bounds where it did nothing, `Fill` and `Clamp` carried a
pass-through note for it, and `Equal` compared it. The roadmap item asked
for the occurrence time to leave the delivery settings and for the storage
shape to be chosen: keep the JSON key, or a column.

A first build made the column nullable, NULL on every message no schedule
produced. That needed `NULLIF` on the write and `COALESCE` on three claim
reads, and the zero `MessageMeta.ScheduledAt` was a trap: no code or lab
branched on it, and the playground handler formatted it unguarded.

## Decision

- `ScheduledAt` moves to `ProduceOptions`. `MessageOptions` holds
  delivery settings only; `MessageMeta.ScheduledAt` is unchanged.
- `message_log` gains `scheduled_at TIMESTAMPTZ NOT NULL`: the time the
  message is for. A schedule's message carries the due time, a manual
  `Run` the moment it ran, and any other produce the moment of the
  produce unless the producer set the field. Any producer may set it;
  scheduler-only was a separate decision and stays open.
- The default resolves once at the produce controller beside the
  idempotency key, so a retried append reuses the same instant. It trails
  `created_at` by clock skew between the producer host and Postgres.
- Claims and `ListMessages` read the column plainly; the JSON key is gone.
  Pre-v1, the baseline DDL changed in place; both registries stay empty.
- Keeping the JSON key was rejected: the struct that marshals `options`
  would still carry the field, so the split would exist on the Go surface
  only, and every claim scanning `options` would need a second struct.

## Consequences

`MessageMeta.ScheduledAt` is never zero. "This came from a schedule" is
the message key, not the time. With a value on every message, the name
`scheduled_at` overstates an ordinary produce; the rename is its own
ROADMAP item (the user does not like `occurred_at`). The delivery consumer
attaches no `MessageMeta` today and is untouched.
