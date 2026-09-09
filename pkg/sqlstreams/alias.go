package sqlstreams

// Every type a user spells through this package is an alias into the
// package that declares it, so a click-through lands on the declaration.

import (
	"github.com/agentstax/sqlstreams/pkg/admin"
	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/consume"
	"github.com/agentstax/sqlstreams/pkg/consumer"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/produce"
	"github.com/agentstax/sqlstreams/pkg/produce/batcher"
	"github.com/agentstax/sqlstreams/pkg/producer"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	"github.com/agentstax/sqlstreams/pkg/scheduler"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/agentstax/sqlstreams/pkg/system"
	"github.com/agentstax/sqlstreams/pkg/worker"
)

type (
	Versioned                        = common.Versioned
	RawPayload                       = common.RawPayload
	StoredMessage[Message Versioned] = common.StoredMessage[Message]
	MessageOptions                   = common.MessageOptions
	RetryPolicy                      = common.RetryPolicy
	ConcurrencyPolicy                = common.ConcurrencyPolicy
	Owner                            = common.Owner
	OwnerKind                        = common.OwnerKind
	Logger                           = logging.Logger
	DiagnosticError                  = diagnostic.DiagnosticError
	DiagnosticEvent                  = diagnostic.DiagnosticEvent
	DiagnosticQuery                  = diagnostic.DiagnosticQuery
	DiagnosticRecovery               = diagnostic.DiagnosticRecovery
	DiagnosticKind                   = diagnostic.DiagnosticKind

	Querier         = datastore.Querier
	Tx              = datastore.Tx
	TransactionFunc = datastore.TransactionFunc

	ProduceOptions                   = produce.ProduceOptions
	CompactionOptions                = produce.CompactionOptions
	ProducerFunc[Message Versioned]  = produce.ProducerFunc[Message]
	BatcherConfig                    = batcher.BatcherConfig
	ProducerConfig                   = producer.ProducerConfig
	MetricProducerInstance           = producer.MetricProducerInstance
	ProduceItem[Message Versioned]   = producer.ProduceItem[Message]
	ProduceResult[Message Versioned] = producer.ProduceResult[Message]

	ConsumerConfig                  = consumer.ConsumerConfig
	ConsumeOptions                  = consumer.ConsumeOptions
	ConsumerFunc[Message Versioned] = consumer.ConsumerFunc[Message]
	CursorPosition                  = consume.CursorPosition
	CursorPositionKind              = consume.CursorPositionKind
	Consumer                        = consume.Consumer
	Binding                         = consume.Binding
	BindingOutcome                  = consume.BindingOutcome
	MessageMeta                     = consume.MessageMeta

	StreamConfig    = stream.StreamConfig
	Stream          = stream.Stream
	DeliveryLogMode = stream.DeliveryLogMode

	SchedulerConfig              = scheduler.SchedulerConfig
	ScheduleRunOptions           = scheduler.ScheduleRunOptions
	Schedule                     = schedule.Schedule
	ScheduleConsumerGroupSummary = schedule.ScheduleConsumerGroupSummary
	ScheduleMessageStatus        = schedule.ScheduleMessageStatus
	ScheduleMessageOutcome       = schedule.ScheduleMessageOutcome
	ScheduleStoredMessage        = schedule.ScheduleStoredMessage

	System         = system.System
	Worker         = worker.Worker
	InstanceTarget = worker.InstanceTarget

	DestroyOptions      = admin.DestroyOptions
	SystemConfig        = system.SystemConfig
	StreamVersionHealth = stream.StreamVersionHealth

	PartitionCountAlertConfig          = alert.PartitionCountAlertConfig
	CompactionReadCostAlertConfig      = alert.CompactionReadCostAlertConfig
	WorkerLivenessAlertConfig          = alert.WorkerLivenessAlertConfig
	MetricCollectorProgressAlertConfig = alert.MetricCollectorProgressAlertConfig
	MetricCollectorWorkerConfig        = metric.MetricCollectorWorkerConfig

	StreamSnapshot                = metric.StreamSnapshot
	ConsumerGroupSnapshot         = metric.ConsumerGroupSnapshot
	StreamSchemaVersionSnapshot   = metric.StreamSchemaVersionSnapshot
	ConsumerGroupSchemaVersionLag = metric.ConsumerGroupSchemaVersionLag
	ConsumerGroupLag              = metric.ConsumerGroupLag
	CursorSnapshot                = metric.CursorSnapshot
	ExceptionSnapshot             = metric.ExceptionSnapshot
	AbandonedRoutineSnapshot      = metric.AbandonedRoutineSnapshot
	Measurement                   = metric.Measurement
	MetricDefinition              = metric.MetricDefinition
	MetricKind                    = metric.MetricKind
	MetricScope                   = diagnostic.MetricScope
	MetricUnit                    = metric.MetricUnit
	Alert                         = alert.Alert
	AlertDefinition               = alert.AlertDefinition
	AlertStatus                   = alert.AlertStatus
	AlertSeverity                 = alert.AlertSeverity
	AlertEvaluationSnapshot       = alert.AlertEvaluationSnapshot
	AlertEvaluationState          = alert.AlertEvaluationState
)

const (
	AlertEvaluationStateHealthy              = alert.AlertEvaluationStateHealthy
	AlertEvaluationStatePending              = alert.AlertEvaluationStatePending
	AlertEvaluationStateActive               = alert.AlertEvaluationStateActive
	AlertEvaluationStateInsufficientEvidence = alert.AlertEvaluationStateInsufficientEvidence

	RecoveryTransient = diagnostic.RecoveryTransient
	RecoveryPermanent = diagnostic.RecoveryPermanent

	DiagnosticKindError  = diagnostic.DiagnosticKindError
	DiagnosticKindEvent  = diagnostic.DiagnosticKindEvent
	DiagnosticKindMetric = diagnostic.DiagnosticKindMetric
	DiagnosticKindAlert  = diagnostic.DiagnosticKindAlert

	ConcurrencyParallel  = common.ConcurrencyParallel
	ConcurrencyExclusive = common.ConcurrencyExclusive
	ConcurrencyOrdered   = common.ConcurrencyOrdered

	DeliveryLogModeOff      = stream.DeliveryLogModeOff
	DeliveryLogModeFailures = stream.DeliveryLogModeFailures
	DeliveryLogModeAll      = stream.DeliveryLogModeAll

	OwnerAny           = common.OwnerAny
	OwnerSystem        = common.OwnerSystem
	OwnerStream        = common.OwnerStream
	OwnerConsumerGroup = common.OwnerConsumerGroup

	CursorPositionBeginning = consume.CursorPositionBeginning
	CursorPositionHead      = consume.CursorPositionHead
	BindingInstalled        = consume.BindingInstalled
	BindingJoined           = consume.BindingJoined
	BindingWaiting          = consume.BindingWaiting

	ScheduleMessagePending    = schedule.ScheduleMessagePending
	ScheduleMessageDeferred   = schedule.ScheduleMessageDeferred
	ScheduleMessageSucceeded  = schedule.ScheduleMessageSucceeded
	ScheduleMessageFailed     = schedule.ScheduleMessageFailed
	ScheduleMessageSuperseded = schedule.ScheduleMessageSuperseded

	NoInstanceTarget = worker.NoInstanceTarget

	MetricKindCounter          = metric.MetricKindCounter
	MetricKindGauge            = metric.MetricKindGauge
	MetricScopeSystem          = diagnostic.MetricScopeSystem
	MetricScopeStream          = diagnostic.MetricScopeStream
	MetricScopeConsumerGroup   = diagnostic.MetricScopeConsumerGroup
	MetricScopeConsumerSession = diagnostic.MetricScopeConsumerSession
	MetricScopeExporter        = diagnostic.MetricScopeExporter
	MetricUnitMilliseconds     = metric.MetricUnitMilliseconds
	AlertStatusActive          = alert.AlertStatusActive
	AlertStatusResolved        = alert.AlertStatusResolved
	AlertSeverityWarn          = alert.AlertSeverityWarn

	MetricStreamName   = metric.MetricStreamName
	ScheduleStreamName = schedule.ScheduleStreamName
	AlertStreamName    = alert.AlertStreamName
)

var (
	LifecycleContext = common.LifecycleContext
	MetaFromContext  = consume.MetaFromContext
	Terminal         = consume.Terminal
	Delay            = consume.Delay
	Beginning        = consume.Beginning
	Head             = consume.Head
	NewMeasurement   = metric.NewMeasurement
)
