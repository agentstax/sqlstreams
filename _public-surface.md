# Supported public API review

Working review, 2026-09-06. Boundary accepted in [0665]; rows distinguish
implemented changes from remaining proposals. Replaces the obsolete 2026-08-01 inventory.
Delete this file at close-out after verdicts are recorded in docs/decisions/.

## Boundary

The entry package is currently `pkg/vulkan`. Its exported names and the
exported fields and methods reachable through them form the supported API.
Other packages remain importable for advanced use, without a stability
commitment or alternative-entry-point guides. Alias comments remain at their
declarations. Third-party types retain their upstream contracts.

The package's eventual public location is not selected. Keep relocation
separate from semantic changes so import changes do not obscure API changes.

## Decisions to review first

Keep means retain the current capability. Question means settle its public
contract before v1. Removed records a decision implemented in this review.

| Verdict | Surface | Recommendation and consequence |
| --- | --- | --- |
| Removed (review in progress) | Nested `SystemConfig` stub | Empty configuration offered no choice. The real alert and collector declaration is now `SystemConfig`; registration creates the singleton directly. No settings or stored rows changed. |
| Renamed (review in progress) | `ErrConsumerNotFound`, `ErrConsumerGroupLive`, `ErrConsumerGroupDeliveriesPending`, `ScheduleConsumerGroupSummary`, `Binding.ConsumerGroupName` | Applied the consumer resource / consumer-group shared-state distinction at the declarations and callers. Binding and schedule JSON use `consumer_group`; the CLI schedule summary collection uses `consumer_groups`. Diagnostic codes VK0014–VK0016 and log attribute keys are unchanged. Old Go names and JSON keys are replaced without compatibility aliases. |
| Question | `Client.Datastore()`, `PostgresDatastore` alias | Reassess after the otel refactor: the OTel Metrics and Exporter constructors take ctx, pool, and config, constructing their own datastore. Metric production and consumption now use the existing client through System().Metrics(). The exporter no longer requires a shared datastore. CLI's existing connection helper still exposes its pool through Client.Datastore; advanced controllers and labs also use the accessor. These remaining uses need review before deciding keep/remove; no datastore access contract is settled. |
| Question | `Client.Config`, `Client.Logger` | Exported mutable state can imply live reconfiguration, but different components retain different resolved values. Decide whether these are supported read access, live controls, or construction details. Trace mutations before promising behavior; do not silently make existing assignments ineffective. |
| Keep | `ProducerConfig.Batch`, `BatcherConfig` | MaxSize, ConcurrencyLimit, AttemptTimeout, and ShutdownGrace express caller-visible batching and cancellation tradeoffs. Their implementation-package location is not a reason to remove them. |
| Keep | `Tx`, `Querier`, `TransactionFunc`, `Tx.Raw`, InTransaction/InTx methods | These let callers compose SQL and production atomically. Raw explicitly permits pgx access. Keep the transaction ownership and no-automatic-retry contracts visible. |
| Question | `RetryPolicy.CalculateDelay`, `CalculateTotalDelay`, `Equal` | Useful calculations, but aliasing also commits to their semantics. Recommend keeping unless a concrete simplification earns a change. NewDefaultRetryPolicy and RetryableFunc are advanced-only and need no demotion. |
| Question | `MessageOptions.Fill`, `Clamp`, `ResolveConcurrency`, `Equal`, `ScheduledAt` | Resolution methods and scheduler-written metadata share the public options type. Determine which are caller operations versus internal computation. Removing a method from vulkan while retaining a type alias is impossible; changing the declaring method affects internal callers too. Avoid a wrapper hierarchy solely to hide methods. |
| Question | `Owner.IdColumns`, `SystemIdColumn`, `TopicIdColumn`, `ConsumerGroupIdColumn` | Database-column helpers are exposed with an otherwise useful identity read-model. Recommend keeping Owner fields and Kind; investigate relocating SQL-specific helpers only if the internal call-site delta remains small. |
| Question | Diagnostic aliases' `Diagnose` methods and mutable declaration fields | Error matching, codes, values, and documentation links serve users. Declaration-building methods expose more than inspection, and global error/event variables point at shared declarations. Settle mutation expectations; preserve errors.Is and log filtering. |
| Keep | Configuration WithDefaults/Validate and enum validation | Existing caller-visible default/validation pattern. Review correctness and documentation, not export removal by default. |
| Keep | Metrics, alerts, Worker/Owner/InstanceTarget read-models | Operators need these through the public handles. Do not demote merely because worker or metrics packages own declarations. |
| Question | `Worker.Metadata any` | Keep observability, but specify what shape callers may rely on. A supported field does not automatically make every implementation metadata JSON shape a stable control API. |
| Keep | `Topic.Migrate`, `System.Migrate`, `System.MigrateTopics` | Different resources/scopes, not redundant spellings. Migration registries stay importable advanced options. |
| Keep | Current topic/group/system/schedule Destroy methods | Distinct resource operations. DestroyTopicVersion and AlterSystem are not current vulkan methods; no consolidation based on those stale examples. |

No proposed removal should silently change runtime behavior. Before building,
show the old call and its replacement, identify source-compatibility breaks,
and preserve operator visibility for changes to configuration or diagnostics.

## Entry-package inventory

Implemented during review: SystemMetricsHandle.Producer returns a
MetricsProducerHandle; Register returns the aliased producer.MetricsProducerInstance.
SystemMetricsHandle.Consumer(name) returns the existing ConsumerHandle[Measurement].
NewMeasurement aliases metrics.NewMeasurement. Core producer logic retains metric
routing, series compaction, and the reserved-name guard. The otel module retains
only Metrics and Exporter; consumer upkeep follows the client's normal lifecycle.

Implemented during review: admin consumer operations use GetConsumer,
ListConsumers, ListConsumerWorkers, and DestroyConsumer; shared ownership
and metrics use ConsumerGroupOwner and ConsumerGroupMetrics. CLI commands
use `consumer` and alert selectors use `--consumer`. Consumer config and
destroy JSON documents name the selected resource `consumer`. Destroy still
deletes the shared registration for all its instances. These names replace
the old admin methods, `group` command, `--group` flag, and those documents'
`group` key; no compatibility aliases were added. Final rationale belongs
in the single review decision record.

All non-test Go files in pkg/vulkan were scanned for exported declarations.
Rows group related names for review; a Question row overrides the Keep default
only for the candidate named above. This is a source inventory, not a claim
that every method's behavior has received a correctness audit.

| Source | Current exported declarations | Verdict |
| --- | --- | --- |
| [alert.go](pkg/vulkan/alert.go) | `AlertHandle`, `AlertHandle.Latest`, `AlertHandle.History` | Keep |
| [binding.go](pkg/vulkan/binding.go) | `BindingHandle`, `SystemHandle.Bindings`, `ConsumerHandle.Binding`, `BindingHandle.Get` | Keep |
| [client.go](pkg/vulkan/client.go) | `Client`, `NewClient`, `Client.Datastore`, `Client.InTransaction` | Question: shared mutable state; keep other verbs |
| [client_config.go](pkg/vulkan/client_config.go) | `ClientConfig`, `ClientConfig.WithDefaults`, `ClientConfig.Validate` | Keep |
| [consumer.go](pkg/vulkan/consumer.go) | `ConsumerHandle`, `TopicHandle.Consumers`, `TopicHandle.Consumer`, `ConsumerHandle.Register`, `ConsumerHandle.Get`, `ConsumerHandle.Workers`, `ConsumerHandle.Destroy` | Keep |
| [consumer_alerts.go](pkg/vulkan/consumer_alerts.go) | `ConsumerAlertsHandle`, `ConsumerHandle.Alerts`, `ConsumerAlertsHandle.Definitions`, `ConsumerAlertsHandle.Latest`, `ConsumerAlertsHandle.Alert` | Keep |
| [consumer_instance.go](pkg/vulkan/consumer_instance.go) | `ConsumerInstance`, `ConsumerInstance.Consume` | Keep |
| [consumer_metrics.go](pkg/vulkan/consumer_metrics.go) | `ConsumerMetricsHandle`, `ConsumerHandle.Metrics`, `ConsumerMetricsHandle.Definitions`, `ConsumerMetricsHandle.Snapshot`, `ConsumerMetricsHandle.CursorHead`, `ConsumerMetricsHandle.CursorClaimed`, `ConsumerMetricsHandle.CursorCommitted`, `ConsumerMetricsHandle.CursorBacklog`, `ConsumerMetricsHandle.CursorInflight`, `ConsumerMetricsHandle.ReadyExceptions`, `ConsumerMetricsHandle.InflightExceptions`, `ConsumerMetricsHandle.DeferredExceptions`, `ConsumerMetricsHandle.DeadExceptions`, `ConsumerMetricsHandle.OldestUnresolvedAge`, `ConsumerMetricsHandle.OpenLeases`, `ConsumerMetricsHandle.AbandonedRoutinesOutstanding`, `ConsumerMetricsHandle.AbandonedRoutinesTotal`, `ConsumerMetricsHandle.AbandonedRoutinesSelfClearLatencyAverage` | Keep |
| [key.go](pkg/vulkan/key.go) | `KeyHandle`, `TopicHandle.Key`, `KeyHandle.CompactionHead`, `KeyHandle.LockCompactionHead`, `KeyHandle.Messages` | Keep |
| [manager.go](pkg/vulkan/manager.go) | `ManagerHandle`, `Client.Manager`, `ManagerHandle.Run` | Keep |
| [metric.go](pkg/vulkan/metric.go) | `MetricHandle`, `MetricHandle.Latest`, `MetricHandle.History` | Keep |
| [pool.go](pkg/vulkan/pool.go) | `NewPostgresPool` | Keep |
| [postgres_connection_config.go](pkg/vulkan/postgres_connection_config.go) | `PostgresConnectionConfig`, `PostgresConnectionConfig.WithDefaults`, `PostgresConnectionConfig.Validate` | Keep |
| [metrics_producer.go](pkg/vulkan/metrics_producer.go) | `MetricsProducerHandle`, `MetricsProducerHandle.Register` | Keep |
| [producer.go](pkg/vulkan/producer.go) | `ProducerHandle`, `TopicHandle.Producer`, `ProducerHandle.Register` | Keep |
| [producer_instance.go](pkg/vulkan/producer_instance.go) | `ProducerInstance`, `ProducerInstance.Produce`, `ProducerInstance.ProduceBatch`, `NewProduceItem`, `ProducerInstance.ProduceFunc`, `ProducerInstance.ProduceInTx`, `ProducerInstance.ProduceFuncInTx` | Keep |
| [scheduler.go](pkg/vulkan/scheduler.go) | `SchedulerHandle`, `Client.Schedulers`, `Client.Scheduler`, `SchedulerHandle.Register`, `SchedulerHandle.Get`, `SchedulerHandle.Suspend`, `SchedulerHandle.Unsuspend`, `SchedulerHandle.Run`, `SchedulerHandle.Status`, `SchedulerHandle.Messages`, `SchedulerHandle.Destroy` | Keep |
| [scheduler_instance.go](pkg/vulkan/scheduler_instance.go) | `SchedulerInstance`, `SchedulerInstance.Schedule` | Keep |
| [system.go](pkg/vulkan/system.go) | `SystemHandle`, `Client.System`, `SystemHandle.Register`, `SystemHandle.Get`, `SystemHandle.Migrate`, `SystemHandle.MigrationVersion`, `SystemHandle.MigrateTopics`, `SystemHandle.Destroy` | Keep |
| [system_alerts.go](pkg/vulkan/system_alerts.go) | `SystemAlertsHandle`, `SystemHandle.Alerts`, `SystemAlertsHandle.Definitions`, `SystemAlertsHandle.Latest`, `SystemAlertsHandle.Alert` | Keep |
| [system_metrics.go](pkg/vulkan/system_metrics.go) | `SystemMetricsHandle.Producer`, `SystemMetricsHandle.Consumer`, `SystemMetricsHandle`, `SystemHandle.Metrics`, `SystemMetricsHandle.Definitions`, `SystemMetricsHandle.Latest`, `SystemMetricsHandle.Metric`, `SystemMetricsHandle.UnclaimedWorkers`, `SystemMetricsHandle.OldestUnclaimedAge`, `SystemMetricsHandle.FailingWorkers`, `SystemMetricsHandle.OverdueSchedules`, `SystemMetricsHandle.OldestDueAge`, `SystemMetricsHandle.SuspendedSchedules`, `SystemMetricsHandle.ActiveAlerts`, `SystemMetricsHandle.ResolvedAlerts`, `SystemMetricsHandle.CheckTopicsEvaluated`, `SystemMetricsHandle.CheckTopicsFailed`, `SystemMetricsHandle.CheckPublishedAlerts`, `SystemMetricsHandle.CheckResolvedAlerts` | Keep |
| [topic.go](pkg/vulkan/topic.go) | `TopicHandle`, `Client.Topics`, `Client.Topic`, `TopicHandle.Register`, `TopicHandle.Get`, `TopicHandle.Migrate`, `TopicHandle.MigrationVersion`, `TopicHandle.Rename`, `TopicHandle.Destroy`, `TopicHandle.Health`, `TopicHandle.CompactionHeads` | Keep |
| [topic_alerts.go](pkg/vulkan/topic_alerts.go) | `TopicAlertsHandle`, `TopicHandle.Alerts`, `TopicAlertsHandle.Definitions`, `TopicAlertsHandle.Latest`, `TopicAlertsHandle.Alert`, `TopicAlertsHandle.PartitionCount`, `TopicAlertsHandle.CompactionReadCost`, `TopicAlertsHandle.WorkerLiveness` | Keep |
| [topic_metrics.go](pkg/vulkan/topic_metrics.go) | `TopicMetricsHandle`, `TopicHandle.Metrics`, `TopicMetricsHandle.Definitions`, `TopicMetricsHandle.Snapshot`, `TopicMetricsHandle.Compacted` | Keep |

## Aliased types and their methods

Every type alias in alias.go is listed here, with exported methods found on
its declaring type. Fields remain part of the review through the linked
source; the first table identifies field-level candidates. Rows include the
SystemConfig and ScheduleRunOptions changes already implemented. A method on an aliased type is supported even when no
free constructor for that type is exported by vulkan.

| Alias | Declaration | Exported methods | Verdict |
| --- | --- | --- | --- |
| `Versioned` | [common.Versioned](pkg/common/versioned.go) | `SchemaVersion` (interface) | Keep |
| `RawPayload` | [common.RawPayload](pkg/common/raw_payload.go) | `SchemaVersion`, `MarshalJSON`, `UnmarshalJSON` | Keep |
| `StoredMessage` | [common.StoredMessage](pkg/common/message.go) | None declared | Keep |
| `MessageOptions` | [common.MessageOptions](pkg/common/message_options.go) | `Fill`, `Clamp`, `ResolveConcurrency`, `Equal`, `WithDefaults`, `Validate` | Question |
| `RetryPolicy` | [common.RetryPolicy](pkg/common/retry_policy.go) | `CalculateDelay`, `CalculateTotalDelay`, `Equal`, `WithDefaults`, `Validate` | Question |
| `ConcurrencyPolicy` | [common.ConcurrencyPolicy](pkg/common/concurrency_policy.go) | `Validate`, `HoldsKey` | Keep |
| `Owner` | [common.Owner](pkg/common/owner.go) | `Kind`, `IdColumns`, `SystemIdColumn`, `TopicIdColumn`, `ConsumerGroupIdColumn` | Question |
| `OwnerKind` | [common.OwnerKind](pkg/common/owner.go) | `Validate` | Keep |
| `Logger` | [logging.Logger](pkg/common/logging/logger.go) | `DebugContext`, `InfoContext`, `WarnContext`, `ErrorContext` (interface) | Keep |
| `DiagnosticError` | [diagnostic.DiagnosticError](pkg/common/diagnostic/error.go) | `Diagnose`, `FixPlaceholders`, `Fill`, `With`, `Wrap`, `Values`, `Unwrap`, `Error`, `Is`, `LogValue`, `Docs`, `GetCode`, `GetKind` | Question |
| `DiagnosticEvent` | [diagnostic.DiagnosticEvent](pkg/common/diagnostic/event.go) | `Diagnose`, `Docs`, `GetCode`, `GetKind` | Question |
| `DiagnosticQuery` | [diagnostic.DiagnosticQuery](pkg/common/diagnostic/query.go) | `Placeholders` | Keep |
| `DiagnosticRecovery` | [diagnostic.DiagnosticRecovery](pkg/common/diagnostic/error.go) | None declared | Keep |
| `DiagnosticKind` | [diagnostic.DiagnosticKind](pkg/common/diagnostic/registry.go) | None declared | Keep |
| `PostgresDatastore` | [datastore.PostgresDatastore](pkg/datastore/datastore.go) | None declared | Question |
| `Querier` | [datastore.Querier](pkg/datastore/querier.go) | `Exec`, `Query`, `QueryRow`, `SendBatch`, `CopyFrom` (interface) | Keep |
| `Tx` | [datastore.Tx](pkg/datastore/transaction.go) | `Raw`, embedded `Querier` methods (interface) | Keep |
| `TransactionFunc` | [datastore.TransactionFunc](pkg/datastore/transaction.go) | None declared | Keep |
| `ProduceOptions` | [produce.ProduceOptions](pkg/produce/options.go) | `Validate` | Keep |
| `CompactionOptions` | [produce.CompactionOptions](pkg/produce/options.go) | `Validate` | Keep |
| `ProducerFunc` | [produce.ProducerFunc](pkg/produce/producer_func.go) | None declared | Keep |
| `BatcherConfig` | [batcher.BatcherConfig](pkg/produce/batcher/batcher_config.go) | `WithDefaults`, `Validate` | Keep |
| `MetricsProducerInstance` | [producer.MetricsProducerInstance](pkg/producer/metrics_producer_instance.go) | `Produce` | Keep |
| `ProducerConfig` | [producer.ProducerConfig](pkg/producer/producer_config.go) | `WithDefaults`, `Validate` | Keep |
| `ProduceItem` | [producer.ProduceItem](pkg/producer/produce_item.go) | None declared | Keep |
| `ProduceResult` | [producer.ProduceResult](pkg/producer/produce_result.go) | None declared | Keep |
| `ConsumerConfig` | [consumer.ConsumerConfig](pkg/consumer/consumer_config.go) | `WithDefaults`, `Validate`, `DeepCopy` | Keep |
| `ConsumeOptions` | [consumer.ConsumeOptions](pkg/consumer/consume_options.go) | `WithDefaults`, `Validate` | Keep |
| `ConsumerFunc` | [consumer.ConsumerFunc](pkg/consumer/consumer.go) | None declared | Keep |
| `CursorPosition` | [consume.CursorPosition](pkg/consume/cursor_position.go) | None declared | Keep |
| `CursorPositionKind` | [consume.CursorPositionKind](pkg/consume/cursor_position.go) | `Validate` | Keep |
| `Consumer` | [consume.Consumer](pkg/consume/consumer.go) | None declared | Keep |
| `Binding` | [consume.Binding](pkg/consume/binding.go) | None declared | Keep |
| `BindingOutcome` | [consume.BindingOutcome](pkg/consume/binding.go) | None declared | Keep |
| `MessageMeta` | [consume.MessageMeta](pkg/consume/meta.go) | None declared | Keep |
| `TopicConfig` | [topic.TopicConfig](pkg/topic/topic_config.go) | `WithDefaults`, `Validate`, `ToTopic` | Keep |
| `Topic` | [topic.Topic](pkg/topic/topic.go) | None declared | Keep |
| `DeliveryLogMode` | [topic.DeliveryLogMode](pkg/topic/topic.go) | None declared | Keep |
| `SchedulerConfig` | [scheduler.SchedulerConfig](pkg/scheduler/scheduler_config.go) | `WithDefaults`, `Validate` | Keep |
| `Schedule` | [schedule.Schedule](pkg/schedule/schedule.go) | None declared | Keep |
| `ScheduleConsumerGroupSummary` | [schedule.ScheduleConsumerGroupSummary](pkg/schedule/consumer_group_summary.go) | None declared | Keep |
| `ScheduleMessageStatus` | [schedule.ScheduleMessageStatus](pkg/schedule/message_status.go) | None declared | Keep |
| `ScheduleMessageOutcome` | [schedule.ScheduleMessageOutcome](pkg/schedule/message_status.go) | None declared | Keep |
| `ScheduleStoredMessage` | [schedule.ScheduleStoredMessage](pkg/schedule/stored_message.go) | `SchemaVersion`, `MarshalJSON` | Keep |
| `System` | [system.System](pkg/system/system.go) | None declared | Keep |
| `Worker` | [worker.Worker](pkg/worker/worker.go) | None declared | Question |
| `InstanceTarget` | [worker.InstanceTarget](pkg/worker/worker.go) | `Suspended`, `Validate` | Keep |
| `DestroyOptions` | [admin.DestroyOptions](pkg/admin/topic.go) | None declared | Keep |
| `SystemConfig` | [system.SystemConfig](pkg/system/system_config.go) | `WithDefaults`, `Validate` | Keep; domain-owned declaration, composed from alert and metrics configs; admin retains registration orchestration |
| `ScheduleRunOptions` | [scheduler.ScheduleRunOptions](pkg/scheduler/schedule_run_options.go) | `WithDefaults`, `Validate` | Keep; moved to the owning scheduler assembler; subject precedes operation in qualified command input names |
| `TopicVersionHealth` | [admin.TopicVersionHealth](pkg/admin/health.go) | None declared | Keep |
| `PartitionCountAlertConfig` | [alert.PartitionCountAlertConfig](pkg/alert/alert_config.go) | `WithDefaults`, `Validate` | Keep |
| `CompactionReadCostAlertConfig` | [alert.CompactionReadCostAlertConfig](pkg/alert/alert_config.go) | `WithDefaults`, `Validate` | Keep |
| `WorkerLivenessAlertConfig` | [alert.WorkerLivenessAlertConfig](pkg/alert/alert_config.go) | `WithDefaults`, `Validate` | Keep |
| `MetricsCollectorWorkerConfig` | [metrics.MetricsCollectorWorkerConfig](pkg/metrics/metricscollectorworker_config.go) | `WithDefaults`, `Validate` | Keep |
| `TopicSnapshot` | [metrics.TopicSnapshot](pkg/metrics/topic.go) | None declared | Keep |
| `ConsumerGroupSnapshot` | [metrics.ConsumerGroupSnapshot](pkg/metrics/metrics.go) | `Lag` | Keep |
| `TopicSchemaVersionSnapshot` | [metrics.TopicSchemaVersionSnapshot](pkg/metrics/topic.go) | None declared | Keep |
| `ConsumerGroupSchemaVersionLag` | [metrics.ConsumerGroupSchemaVersionLag](pkg/metrics/topic.go) | None declared | Keep |
| `ConsumerGroupLag` | [metrics.ConsumerGroupLag](pkg/metrics/metrics.go) | None declared | Keep |
| `CursorSnapshot` | [metrics.CursorSnapshot](pkg/metrics/metrics.go) | None declared | Keep |
| `ExceptionSnapshot` | [metrics.ExceptionSnapshot](pkg/metrics/metrics.go) | None declared | Keep |
| `AbandonedRoutineSnapshot` | [metrics.AbandonedRoutineSnapshot](pkg/metrics/abandoned_routine.go) | None declared | Keep |
| `Measurement` | [metrics.Measurement](pkg/metrics/measurement.go) | None declared | Keep |
| `MetricDefinition` | [metrics.MetricDefinition](pkg/metrics/definition.go) | None declared | Keep |
| `MetricKind` | [metrics.MetricKind](pkg/metrics/measurement.go) | `Validate` | Keep |
| `MetricScope` | [diagnostic.MetricScope](pkg/common/diagnostic/metric.go) | `Validate` | Keep |
| `MetricUnit` | [metrics.MetricUnit](pkg/metrics/measurement.go) | `Validate` | Keep |
| `Alert` | [alert.Alert](pkg/alert/alert.go) | `RoutingKey` | Keep |
| `AlertDefinition` | [alert.AlertDefinition](pkg/alert/definition.go) | None declared | Keep |
| `AlertStatus` | [alert.AlertStatus](pkg/alert/alert.go) | None declared | Keep |
| `AlertSeverity` | [alert.AlertSeverity](pkg/alert/alert.go) | None declared | Keep |

`Tx` and `Querier` are interfaces: their method contracts are declared in
[transaction.go](pkg/datastore/transaction.go) and [querier.go](pkg/datastore/querier.go),
including the embedded query methods and Raw. Logger and Versioned likewise
retain their interface methods. External pgx, TLS, context, time, and slog types
are dependencies at the boundary, not Vulkan-owned types to rename or duplicate.

## Constants, errors, events, and function variables

Keep the existing enum values, typed selectors, named errors, diagnostic events,
and convenience functions. They support configuration, errors.Is, and structured
log filtering. Question the mutability of shared diagnostic declarations as
noted above, not their availability to users.

[alias.go](pkg/vulkan/alias.go): `RecoveryTransient`, `RecoveryPermanent`, `DiagnosticKindError`, `DiagnosticKindEvent`, `DiagnosticKindMetric`, `DiagnosticKindAlert`, `ConcurrencyParallel`, `ConcurrencyExclusive`, `ConcurrencyOrdered`, `DeliveryLogModeOff`, `DeliveryLogModeFailures`, `DeliveryLogModeAll`, `OwnerAny`, `OwnerSystem`, `OwnerTopic`, `OwnerConsumerGroup`, `CursorPositionBeginning`, `CursorPositionHead`, `BindingInstalled`, `BindingJoined`, `BindingWaiting`, `ScheduleMessagePending`, `ScheduleMessageDeferred`, `ScheduleMessageSucceeded`, `ScheduleMessageFailed`, `ScheduleMessageSuperseded`, `NoInstanceTarget`, `MetricKindCounter`, `MetricKindGauge`, `MetricScopeSystem`, `MetricScopeTopic`, `MetricScopeConsumerGroup`, `MetricScopeConsumerSession`, `MetricUnitMilliseconds`, `AlertStatusActive`, `AlertStatusResolved`, `AlertSeverityWarn`, `MetricsTopicName`, `ScheduleTopicName`, `AlertTopicName`, `LifecycleContext`, `MetaFromContext`, `Terminal`, `Delay`, `Beginning`, `Head`, `NewCompactionOptions`, `NewMeasurement`.

[errors.go](pkg/vulkan/errors.go): `ErrAlreadyConsuming`, `ErrCommitConfirmationLost`, `ErrLeaseLost`, `ErrLifecycleContextNotCancellable`, `ErrCompactionHeadNotFound`, `ErrDeliveryDelayed`, `ErrDeliveryTerminal`, `ErrConsumerGroupDeliveriesPending`, `ErrConsumerGroupLive`, `ErrConsumerNotFound`, `ErrNotRegistered`, `ErrSchemaNewerThanBuild`, `ErrSchemaOlderThanBuild`, `ErrStepLockTimeout`, `ErrPartitionCreationBehind`, `ErrPartitionLockTimeout`, `ErrScheduleDeclarationInterrupted`, `ErrScheduleNotFound`, `ErrSchemaNotCreatable`, `ErrSystemLive`, `ErrTopicsRegistered`, `ErrDestroyDisabled`, `ErrReservedTopicName`, `ErrTopicConfigMismatch`, `ErrTopicDeclarationInterrupted`, `ErrTopicNameTaken`, `ErrTopicNotEmpty`, `ErrTopicNotFound`, `ErrTopicPartitionsRemain`, `ErrInstanceLost`, `ErrWorkerDeclarationInterrupted`.

[events.go](pkg/vulkan/events.go): `EventAlertConditionHolds`, `EventConsumerStopped`, `EventExceptionDeadLettered`, `EventGroupConfigNotRefreshed`, `EventKillBackstopFired`, `EventLeaseReclaimed`, `EventMessageDeadLettered`, `EventMessagesDeadLettered`, `EventRangeQuarantined`, `EventSlowDispatch`, `EventStoredOptionsClamped`, `EventGoRoutineEventsDropped`, `EventPartitionCreatedOnInsert`, `EventPartitionNotCreatedAhead`, `EventSlowProduce`, `EventMessageAlreadyProduced`, `EventScheduleConfigReplaced`, `EventTargetKeepsNoSuccessRows`, `EventSystemManagerStopped`, `EventTopicConfigReplaced`, `EventInstanceLost`, `EventManagerRowSuspended`, `EventSlowTick`, `EventTickBackoffCurveExhausted`, `EventWorkerConfigReplaced`.

## Old task disposition

| Old item | Current disposition |
| --- | --- |
| Hide concurrency; remove consumer queue/pool constructor arguments | Constructor cleanup already happened; retain importable concurrency for advanced users. |
| Keep sub-consumer and maintenance constructors public | Remain importable advanced APIs; no supported entry-point guides or v1 stability promise. |
| Hide migrations and registries | Drop the hiding proposal; supported migration verbs already live on resource handles. |
| Broad internal moves / datastore interfaces | No blanket relocation or interface introduction. Revisit only a concrete use case. |
| Unexport common retry helpers | Not reachable through vulkan as free functions; no supported-surface benefit. |
| DestroyTopicVersion / AlterSystem examples | Obsolete names; inspect current handles instead. |
| Delete field-less RegisterSystem config | Implemented during this review: removed the empty nested System member; the remaining real settings are `SystemConfig`. |

## Implementation order after review

1. Settle the candidate table, with call-site evidence for each removal.
2. Publish the accepted signature proposals on the doc site, then implement
   the smallest change to the owning declaration and all affected callers.
3. Run alias-closure/conventions checks, builds, targeted race tests, and
   directly affected labs when behavior changes; update public comments.
4. Record shipped verdicts, remove completed TODO/ROADMAP work, and delete
   this inventory once its content is folded into the fixed record surface.

Package relocation follows the chosen destination later. Audit import strings,
module-relative restrictions, code exports, alias closure/declaration checks,
examples, integrations, and site links; do not assume a directory move implies
creating another Go module.
