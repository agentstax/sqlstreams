# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

- **Named-return-params house style — design discussion.**
  - Proposed: unnamed results by default; named results only when a deferred
    function must inspect or change the actual returned value. Always spell
    out return values in functions that return results.
  - Confirm the rule and what “reviewed surface” includes before applying it.
    Current examples occur in produce datastores, message-consumer range state,
    the CLI, and labs; review deferred result use before changing signatures.
  - Once settled, record the decision and update CONVENTIONS.md, then apply
    the rule with targeted verification.

- **Separate scheduled occurrence metadata from delivery options** (from
  ROADMAP Now). Proposal drafted 2026-09-06 as the `## Proposed` section of
  website/src/content/docs/guides/schedules.mdx; review it before any code.
  - Shape: `MessageOptions.ScheduledAt` -> `ProduceOptions.ScheduledAt`;
    message_log gains `scheduled_at TIMESTAMPTZ` (NULL on ordinary
    messages); the `options` document holds delivery settings only;
    `MessageMeta.ScheduledAt` unchanged. Any producer may still set it --
    scheduler-only stays a separate decision.
  - Storage choice: a dedicated column, not the JSON key. Keeping the key
    means the struct that marshals `options` still carries the field, so
    the split would exist on the public surface only and every claim scan
    of `options` would need a second struct.
  - Trace (each site touched when built): produce insert (single and
    batch, one `protectedInsertSQL`) -- new column, `NULLIF` on the zero
    time; schedule producer `produceDue` and `RunSchedule` -- set the
    ProduceOptions field (manual run keeps `time.Now().UTC()`);
    messageconsumer readMessages (fresh and reclaim both call it) and
    exceptionconsumer.claim's two SELECTs -- select `m.scheduled_at` into
    the `*Row` (`*time.Time`, the LastScheduledAt pattern) and pass it to
    `toMessageMeta`; `schedule.keyMessages` -- `m.scheduled_at` replaces
    the JSON cast; `MessageOptions.Equal` drops the instant clause and
    Fill/Clamp comments lose the pass-through note; schedulelab's
    `options->>'scheduled_at'` read; website sandbox create-table.ts
    mirror; table-design.mdx and client.mdx prose.
  - Not changed: `StoredMessage` (`topic key messages`) does not show the
    time today and gains no field; deliveryconsumer scans `options` but
    attaches no `MessageMeta` at all today, so no read is added there
    (pre-existing gap, noted, out of scope).
  - Compatibility: pre-v1, baseline DDL edited in place, dev DBs
    drop+recreate; both registries are empty so no step. Old rows' JSON
    key is ignored by encoding/json.
  - After review: build, `go test -race` on touched packages, schedulelab,
    `just verify`; decision record; HISTORY entry; remove the ROADMAP item.
