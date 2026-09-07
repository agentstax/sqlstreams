package vulkan

// Every type a user spells through this package is an alias into the
// package that declares it, so a click-through lands on the declaration.

import (
	"github.com/agentstax/vulkan/pkg/admin"
	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/consume"
	"github.com/agentstax/vulkan/pkg/consumer"
	"github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/agentstax/vulkan/pkg/produce"
	"github.com/agentstax/vulkan/pkg/produce/batcher"
	"github.com/agentstax/vulkan/pkg/producer"
	"github.com/agentstax/vulkan/pkg/schedule"
	"github.com/agentstax/vulkan/pkg/scheduler"
	"github.com/agentstax/vulkan/pkg/system"
	"github.com/agentstax/vulkan/pkg/topic"
	"github.com/agentstax/vulkan/pkg/worker"
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
	MetricsProducerInstance          = producer.MetricsProducerInstance
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

	TopicConfig     = topic.TopicConfig
	Topic           = topic.Topic
	DeliveryLogMode = topic.DeliveryLogMode

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

	DestroyOptions     = admin.DestroyOptions
	SystemConfig       = system.SystemConfig
	TopicVersionHealth = topic.TopicVersionHealth

	PartitionCountAlertConfig           = alert.PartitionCountAlertConfig
	CompactionReadCostAlertConfig       = alert.CompactionReadCostAlertConfig
	WorkerLivenessAlertConfig           = alert.WorkerLivenessAlertConfig
	MetricsCollectorProgressAlertConfig = alert.MetricsCollectorProgressAlertConfig
	MetricsCollectorWorkerConfig        = metrics.MetricsCollectorWorkerConfig

	TopicSnapshot                 = metrics.TopicSnapshot
	ConsumerGroupSnapshot         = metrics.ConsumerGroupSnapshot
	TopicSchemaVersionSnapshot    = metrics.TopicSchemaVersionSnapshot
	ConsumerGroupSchemaVersionLag = metrics.ConsumerGroupSchemaVersionLag
	ConsumerGroupLag              = metrics.ConsumerGroupLag
	CursorSnapshot                = metrics.CursorSnapshot
	ExceptionSnapshot             = metrics.ExceptionSnapshot
	AbandonedRoutineSnapshot      = metrics.AbandonedRoutineSnapshot
	Measurement                   = metrics.Measurement
	MetricDefinition              = metrics.MetricDefinition
	MetricKind                    = metrics.MetricKind
	MetricScope                   = diagnostic.MetricScope
	MetricUnit                    = metrics.MetricUnit
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

	DeliveryLogModeOff      = topic.DeliveryLogModeOff
	DeliveryLogModeFailures = topic.DeliveryLogModeFailures
	DeliveryLogModeAll      = topic.DeliveryLogModeAll

	OwnerAny           = common.OwnerAny
	OwnerSystem        = common.OwnerSystem
	OwnerTopic         = common.OwnerTopic
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

	MetricKindCounter          = metrics.MetricKindCounter
	MetricKindGauge            = metrics.MetricKindGauge
	MetricScopeSystem          = diagnostic.MetricScopeSystem
	MetricScopeTopic           = diagnostic.MetricScopeTopic
	MetricScopeConsumerGroup   = diagnostic.MetricScopeConsumerGroup
	MetricScopeConsumerSession = diagnostic.MetricScopeConsumerSession
	MetricScopeExporter        = diagnostic.MetricScopeExporter
	MetricUnitMilliseconds     = metrics.MetricUnitMilliseconds
	AlertStatusActive          = alert.AlertStatusActive
	AlertStatusResolved        = alert.AlertStatusResolved
	AlertSeverityWarn          = alert.AlertSeverityWarn

	MetricsTopicName  = metrics.MetricsTopicName
	ScheduleTopicName = schedule.ScheduleTopicName
	AlertTopicName    = alert.AlertTopicName
)

var (
	LifecycleContext = common.LifecycleContext
	MetaFromContext  = consume.MetaFromContext
	Terminal         = consume.Terminal
	Delay            = consume.Delay
	Beginning        = consume.Beginning
	Head             = consume.Head
	NewMeasurement   = metrics.NewMeasurement
)
