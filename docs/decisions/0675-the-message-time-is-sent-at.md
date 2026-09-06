---
status: rejected
date: 2026-09-06
phase: "pre-v1"
---

# The message's own time is sent_at

## Context

[0673] had put the scheduled time on every message row as
`scheduled_at`, which overstated an ordinary produce. A research round
compared the general message-time family (Kafka `timestamp`, CloudEvents
`time`) with the scheduler-only family (Temporal `ScheduleTime`, Quartz
`scheduledFireTime`, GoodJob `cron_at`) and picked `sent_at` beside
`created_at`, Kafka's CreateTime and LogAppendTime split.

## Decision

Rejected the same day, with [0673]. A row "sent" before it was "created"
reads as nonsense, and Vulkan's verb is produce. The name was wrong
because the column was wrong: the scheduled time belongs only to a
schedule's message, and the sparse `options` document already holds it.

## Consequences

Nothing shipped under this record. `MessageOptions.ScheduledAt`,
`options.scheduled_at`, and `MessageMeta.ScheduledAt` are as they were
before [0673]. Do not re-propose a message-log column for this fact.
