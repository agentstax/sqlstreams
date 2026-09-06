# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

- Supported public API review [0665]: boundary settled; `_public-surface.md`
  holds the current inventory and proposed decisions. Review the remove/question
  rows before implementation; fold final verdicts into one decision record and
  delete the working inventory at close-out.
  Client.Datastore and the PostgresDatastore alias are removed; CLI/lab/benchmark
  callers use owned pools and explicit datastores. Continue with the remaining
  review close-out after the retry arithmetic follow-up. Worker.Metadata stays
  public as a documented stored-configuration inspection snapshot.
  Client.Config and Client.Logger are also removed; construction
  captures settings and copies the supplied retry policy.
  Diagnostic error/event declarations now use private fields and read accessors;
  constructors own query values, Queries returns snapshots, and Diagnose is removed.
  Owner retains fields and Kind; SQL column conversion moved to pkg/datastore,
  and the unused IdColumns method is removed.
  RetryPolicy helpers stay public; comments and contract tests are complete.
  Boundary verification found total-delay overflow for valid fields. Review
  arithmetic handling with the producer's added operation allowance before
  closing this item; details are in `_public-surface.md`.

- Table name + column review (pre-v1, last pass before the DDL is expensive
  to change). Automated review 2026-09-06; `tools/conventions` DDL walk
  (kinds, `_at`/`_after`, `_ns`) passes, so everything below is what it does
  not check. Manual review still to come -- these are candidates, not verdicts.
  1. Settle the naming findings (same concept, different name):
     - `message_key_lease.lease_token` -> `token` -- SHIPPED 2026-09-06
       (commit 87d5f27d).
     - `worker_instance.attempts` vs `exception_queue.attempts` -- USER-SETTLED
       2026-09-06: keep `attempts`; a rename adds a term and locks in one
       meaning of the streak.
     - `migration_log.migration_version` -> `version` -- SHIPPED 2026-09-06
       (DDL, sandbox mirror, migrate/system/topic literals, VK0022/VK0023
       queries + codes.json, two labs, storybook sample).
     - `compaction_head.head_id` -> `message_id` -- SHIPPED 2026-09-06 (13
       library files, 11 labs, 4 sandbox mirrors, VK0066 query + codes.json,
       table-design.mdx; 36 labs green on a fresh DB).
     - `declared_at` across the `_config_log` kind -- DROPPED 2026-09-06: it
       means the statement instant in all three; binding_config_log's extra
       `attempted_at` exists only because its declarations retry. The
       migration_log.created_at -> attempted_at replacement was declined.
     - version column width -- SHIPPED 2026-09-06, REVERSED direction: a
       version is an ordinal, so every version column is INTEGER (schedule_config
       and message_log and compaction_head schema_version, migration_log version
       and min_compatible_version); BIGINT stays for ids, `_ns`, sizes, and
       compaction_rank. 18 labs green on a fresh DB.
  2. Settle the shape + order findings:
     - `_config` timestamps -- SHIPPED 2026-09-06 [0667]: every `_config`
       table carries both, every config UPDATE sets updated_at (USER-SETTLED:
       consistency over trail redundancy). Conventions test enforces it.
     - 1:1 cursor tables -- SHIPPED 2026-09-06 [0668], REVERSED direction:
       USER-SETTLED surrogate `id` stays (flexibility), schedule_cursor gained
       one; natural/composite-keyed hot tables untouched. Conventions test.
     - claim_lease leads with `token` -- SHIPPED 2026-09-06: columns and PK now
       (consumer_group_id, token); every claim_lease predicate is a PK-prefix
       seek. 11 consume labs green on a fresh DB.
     - binding_config `pattern` before `pattern_regex` -- SHIPPED 2026-09-06
       (DDL, mirror, the INSERT list + args; binding/routing labs green).
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
