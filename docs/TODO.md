# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

- Supported public API review [0665]: boundary settled; `_public-surface.md`
  holds the current inventory and proposed decisions. Review the remove/question
  rows before implementation; fold final verdicts into one decision record and
  delete the working inventory at close-out.

- Table name + column review (pre-v1, last pass before the DDL is expensive
  to change). Automated review 2026-09-06; `tools/conventions` DDL walk
  (kinds, `_at`/`_after`, `_ns`) passes, so everything below is what it does
  not check. Manual review still to come -- these are candidates, not verdicts.
  1. Settle the naming findings (same concept, different name):
     - `message_key_lease.lease_token` -> `token` (bare rule; `claim_lease.token`,
       `worker_instance.token`; the `lease_` prefix stays only on exception_queue).
     - `worker_instance.attempts` vs `exception_queue.attempts` -- USER-SETTLED
       2026-09-06: keep `attempts`; a rename adds a term and locks in one
       meaning of the streak.
     - `migration_log.migration_version` -> `version` (bare rule; Row already
       maps it to `Version`).
     - `compaction_head.head_id` -> `message_id` (FK columns keep the resource's
       noun).
     - `declared_at` means row-write time in topic_config_log/worker_config_log
       (`DEFAULT now()`) but first-statement time in binding_config_log, where
       `attempted_at` is the write time -> one meaning per name across the
       `_config_log` kind.
     - `schedule_config.schema_version INTEGER` -> BIGINT (message_log,
       compaction_head).
  2. Settle the shape + order findings:
     - `_config` timestamps follow no rule: system both, topic both,
       consumer_group created only, worker/schedule/binding none -> pick one.
     - 1:1 cursor tables differ: consumer_group_cursor has surrogate `id` +
       UNIQUE group id; schedule_cursor uses schedule_id as PK -> pick one.
     - claim_lease leads with `token` (columns + PK); every other per-group
       table leads with consumer_group_id.
     - binding_config lists derived `pattern_regex` before declared `pattern`.
     - schedule_config puts `schema_version` after `payload` (message_log:
       before); `concurrency`/`timeout_ns` sit between `suspended` and `payload`.
     - topic_config_log carries a DEFAULT on `empty_compaction_head_ttl_ns`
       only -- leftover from the additive change; snapshot columns take none.
  3. Settle the drift:
     - case: `default 0` (exception_queue.attempts), `now()` x4 vs `NOW()`.
     - worker_config.name comment lists 'janitor'; real names are
       topic_janitor, consumer_group_janitor, cursor_advancer, schedule_producer.
     - index names mix column-named (`_created_at`, `_message_key`, `_attempt`)
       and purpose-named (`_due`, `_expiry`, `_group`, `_topic`, `_worker`).
     - migration_log.consumer_group_id: both version reads filter it IS NULL and
       no group-scope migration exists -> confirm, then drop column + CHECK
       term (pre-v1 baseline edit, no two-release dance).
  4. Write the decision record for whatever 1-3 settle (column-order rule,
     `_config` timestamp rule, cursor-table shape), amend CONVENTIONS ## Tables,
     extend the `tools/conventions` DDL walk for any newly machine-checkable
     rule.
  5. Apply: baseline DDL in place, `*Row` db tags, every SQL literal, the
     website sandbox mirrors under website/src/components/sandbox/sql/, labs
     that hand-copy queries (grep per moved column), diagnose queries.
     Drop+recreate the dev DB, then full fresh-DB lab suite.
