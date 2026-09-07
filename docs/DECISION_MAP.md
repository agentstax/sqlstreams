# Decision map

Keywords -> decision records. Each line: the terms a question would use,
then the records that settled it (`NNNN-NNNN` is a range and may hold a
stray). Bodies live at `docs/decisions/NNNN-<slug>.md`; status and title
are in `docs/DECISIONS.md`. A new record adds its number to its line.

- claim, lease, cursor, claimed/committed, reclaim, snapshot fence, FOR UPDATE SKIP LOCKED: 0001-0005 0041-0047 0061-0067 0101-0107 0141-0145 0161-0166 0387-0396 0616 0685
- retry, backoff, dead-letter, exception path, work timeout, panic, abandoned goroutine: 0042-0043 0181-0192 0281-0294 0399-0400 0614-0615 0670
- routing key, bindings, fan-out, wildcard, binding_log TTL: 0201-0208 0242 0389-0393 0511 0573
- partitions, retention, janitor, create-ahead, heal, drop floor: 0221-0227 0378-0384 0428 0512-0513 0620 0659-0660 0662-0663
- topic catalog, schema version, rename, alter, per-topic tables, table names: 0241-0248 0348-0353 0401-0411 0570-0572 0611 0613 0618 0624 0628 0667 0668 0669
- compaction, message key, rank, ordered/exclusive/parallel, key lease, deadlock: 0261-0273 0403 0463 0574 0612 0617 0659-0660 0691
- produce, ProduceInTx, batch, idempotency key, uuid: 0021-0023 0283-0284 0321-0323 0376 0525 0622-0623 0634
- synchronous_commit, crash lab, bench method, reliability lab, ledger and checker, scenario file, drain, safety checks: 0081-0086 0687 0693
- errors (VK codes, fix, diagnose), logging (levels, buffer, suppression, stop line), metrics declarations, collector poll rate, export freshness, otel, Prometheus, payload never logged: 0302-0305 0326 0522-0524 0550-0554 0558-0569 0589-0590 0647-0648 0661 0666 0670 0678 0682-0684 0686 0688 0689
- migrations, schema versioning, advisory lock, MinCompatibleVersion, Postgres schema/search_path: 0341-0347 0501 0526-0527 0579-0580 0588 0629-0632 0650
- workers, worker_instance, system manager, liveness, instance target, Run loop, fatal consumption errors: 0421-0431 0537 0545-0549 0627 0635-0642 0671
- schedules, cron, missed runs, job status: 0461-0473 0621
- alerts, checks, __system.alerts, repeat interval, history-derived pending, collector progress, restart gaps, bounded history, storage time: 0481-0490 0516 0520 0627 0649 0658 0683-0684 0686 0688 0689 0690 0692 0693 0694
- circuit breaker, error_class, reconciliation: 0502-0506
- packages, layers, seams, naming, receivers, file layout, configs, constructors, admin, validation, consumer worker selection, health: 0441-0451 0507-0510 0528-0549 0555-0557 0643-0646 0657-0658 0670 0676 0677 0680
- declarations: newest-wins, CLI never writes config, worker metadata: 0515-0521 0626 0670
- client shape, handles, Register, lifecycle ctx, shutdown, named return parameters: 0361-0377 0625 0633-0646 0657 0664-0665 0670 0672
- CLI: nested module, flags, --output json: 0354-0355 0576
- doc site: board, sandbox, versioning, voice, cookie, errors, links, page size, reference board: 0581-0610 0651 0677 0679
- rejected/reverted (do not re-suggest): 0270 latest_key backfill, 0379 PartitionsAhead, 0591 pglite prefetch, 0594 byte ceilings, 0672 mandatory named client results, 0673 0675 scheduled time as a message_log column / sent_at, 0323 a library retry inside InTransaction, 0536 a never-nil MessageOptions (NULLIF/COALESCE reshape), 0578 any fillfactor change without measured HOT-ratio degradation, 0626 strict declaration forms (RequireMatch, a stale-build gate)
- playground, examples, handler placement, handle and instance names: 0674
- rule files, record-keeping surface, citations, (checked) markers: 0681
