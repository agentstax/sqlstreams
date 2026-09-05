# TODO

Sliding window of in-flight work only. Future work lives in ROADMAP.md;
shipped work in HISTORY.md; decision rationale in DECISIONS.md ->
docs/decisions/.

## The datastore holds Logger and Retry once [0657]

Invariant: nothing below `vulkan.NewClient` carries a `Logger` or `Retry`
field. The pair lives on `PostgresDatastore`, filled once from
`ClientConfig`, and every layer reads it from the `ds` it already holds.
The facade does not move: `Consumer(name).Register(ctx, *ConsumerConfig)`,
`Producer().Register(ctx, *ProducerConfig)`, `Scheduler(name).Register(ctx,
..., *SchedulerConfig)`, the `Consumer` value and handle all keep their
names and shapes; the three Configs only lose two fields.

Settled this session, do not reopen: renaming the assembler struct
(`ConsumerAssembler` / `Registrar` / `Factory` / `Provisioner`) rejected;
`consumer.Consumer` + `NewConsumer(ds)` stay as they are; [0653]'s
resource name stands; `ds` holds the `RetryPolicy` value and each
datastore keeps building its own `common.NewRetryDatastore`.

Build order, each chunk green (build, `go test -race` on touched
packages, directly affected labs) before the next:

1. **pkg/datastore.** DONE 2026-09-05 (build, race tests, nested modules, conventions, groupconfiglab green). `PostgresDatastoreConfig` gains `Logger` and
   `Retry` after `Schema`; `WithDefaults` fills the stderr warn logger,
   binds the `schema` log attribute (moves from `client.go`; keep its
   pipeline-merge note: bind on a local, never overwrite `cfg.Logger`)
   and the default retry policy; `Validate` checks `Retry`.
   `PostgresDatastore` gains `Logger` and `Retry` fields. `NewClient`
   passes `cfg.Logger` / `cfg.Retry` through and sets `client.Logger =
   ds.Logger`. `tools/compat` and `groupconfiglab` build their own
   datastore and set the pair there.
2. **The three assemblers.** DONE 2026-09-05 (with 3; race tests, conventions, groupconfiglab, schedulelab, alertlab, metricscollectorlab green; interim `retry, logger` params on the two consumer adapters go with chunk 5; `BatcherConfig.Logger` deleted the same day, `NewBatcher` takes the producer instance's logger). `consumer.ConsumerConfig`,
   `producer.ProducerConfig`, `scheduler.SchedulerConfig` drop `Logger`
   and `Retry` (fields, `WithDefaults` blocks, `Validate` check, field
   comments). Each `Register` builds its per-instance pipeline
   (`Buffer`, `Suppress`) over `c.ds.Logger` and passes `c.ds.Retry`
   to the controllers it composes; `ProducerConfig.Batch.Logger` is set
   in `Register` from that logger. `ConsumerInstance` reads
   `i.ds.Retry` where it read `i.Config.Retry` (provisioners, manager).
   `pkg/vulkan/{consumer,producer,scheduler}.go` delete their nil-patch
   blocks; `alias.go` is unchanged (the closure test confirms).
3. **Internal callers.** DONE 2026-09-05 -- also the three alert instances, `schedule/producer`, `metrics/collector`; no lab or playground scenario passed the pair. `admin/scheduler.go`, `admin/system.go`,
   `otelvulkan/metrics_{consumer,producer}.go`, `metrics/collector`,
   `tools/compat`, every lab and playground scenario that passes
   `Logger:`/`Retry:` into one of the three Register calls stops.
4. **`MetricsProducerConfig`.** DONE 2026-09-05 (`NewMetricsProducer(ds, cfg, logger)` takes the owning instance's logger so its warns share that window; metricslab, abandonedeventslab green). `pkg/metrics/producer.ProducerConfig`
   renames to `MetricsProducerConfig` (file
   `metrics_producer_config.go`), keeps `SessionFlushRate`, drops the
   pair. Every caller (`metricslab`, `abandonedeventslab`,
   `abandonedroutinesnapshotlab`, `exclusivelab`, collector) follows.
5. **The follow-on sweep: every config drops the pair.** 63 config
   structs carry it. The 40 that hold nothing else (every
   `ControllerConfig`, every `*DatastoreConfig`) are deleted with their
   file and their constructor's `cfg` param -- `NewTopicController(ds,
   declarers...)`, `NewTopicDatastore(ds)` -- and a datastore builds
   `common.NewRetryDatastore(ds.Retry, ds.Logger)`. The 22 that hold
   other fields (`MessageAdminConfig` -> `{AllowDestroy}`,
   `SystemManagerConfig`, the worker and alert configs with their
   per-loop `SweepRetry`/`TickRetry` curves, `ExporterConfig`,
   `MetricsConfig`) keep those and lose the pair; a worker binds its
   identity `Args` over `ds.Logger`. `Buffer: true` is applied once on
   the datastore's logger; a layer adds only what it composes
   (per-instance `Suppress`, per-operation `WithLogBuffer`). Order:
   datastores, controllers, workers and provisioners, admin and
   systemmanager, then labs. Amend CONVENTIONS ## Constructors & configs:
   the ambient tail exists on `ClientConfig` and
   `PostgresDatastoreConfig` only; per-loop retry curves stay where they
   are.
6. **Docs and records.** `guides/client.mdx` lines 85-89 become true as
   written; check `quickstart.mdx`, `guides/consumer-group-config.mdx`,
   `concepts/lifecycle.mdx` for a `Logger`/`Retry` on a Config below the
   client. Then the review-ready checkpoint: full fresh-DB lab suite,
   `just verify`, `just compat-lab`, HISTORY.md entry citing [0657],
   remove the ROADMAP line, empty this window.
