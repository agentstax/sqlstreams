---
status: accepted
date: 2026-09-06
phase: "pre-v1"
---

# Every _config table carries created_at and updated_at

## Context

The table-name and column review found the six `_config` tables
disagreeing on their timestamp columns: system_config and topic_config
carried both `created_at` and `updated_at`, consumer_group_config only
`created_at`, and worker_config, schedule_config, and binding_config
neither. No rule had produced the split; each table got whatever its
first writer needed. Two of the four UPDATE paths on config rows
(topic rename and replace) set `updated_at`; the schedule and worker
paths had no column to set.

The alternative was the narrower rule: `created_at` everywhere,
`updated_at` only on mutable rows with no `_config_log` trail, since
the newest trail row's `declared_at` already says when a config row
last changed. That saves two columns and avoids a redundant fact on
topic_config and worker_config.

## Decision

Every `_config` table carries `created_at` and `updated_at`, both
`TIMESTAMPTZ NOT NULL DEFAULT NOW()`, as the last two columns before
any table constraint. Consistency across the kind outranks the
redundancy with the `_config_log` trail.

- Every UPDATE on a config row sets `updated_at = NOW()`: topic rename
  and replace, schedule replace, suspend, and unsuspend, the schedule
  producer's missed-run suspend, and the worker metadata replace.
- A config row that is never updated (system_config, and
  binding_config, whose rows are replaced by delete and insert) keeps
  `updated_at` equal to `created_at`.
- Read models are unchanged: a field still needs a production reader.
  System exposes both, Consumer exposes `created_at`, the rest expose
  neither.
- `tools/conventions` walks the baseline DDL and fails on a `_config`
  table missing either column.

## Consequences

Four tables gain columns and five UPDATE statements gain a SET clause;
no Row struct, literal select list, or lab changes, since every insert
names its columns. `_config_log` tables are untouched: they are
append-only and their instant is `declared_at`. schedule_config now
records when it last changed even though it has no trail yet; the
trail stays a roadmap item.
