package sqlstreams

// Every declared log event, under its own name, for callers that filter
// on a code.

import (
	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/consume"
	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/produce"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/agentstax/sqlstreams/pkg/system"
	"github.com/agentstax/sqlstreams/pkg/worker"
)

var (
	EventAlertConditionHolds          = alert.EventAlertConditionHolds
	EventConsumerStopped              = consume.EventConsumerStopped
	EventExceptionDeadLettered        = consume.EventExceptionDeadLettered
	EventGroupConfigNotRefreshed      = consume.EventGroupConfigNotRefreshed
	EventKillBackstopFired            = consume.EventKillBackstopFired
	EventLeaseReclaimed               = consume.EventLeaseReclaimed
	EventMessageDeadLettered          = consume.EventMessageDeadLettered
	EventMessagesDeadLettered         = consume.EventMessagesDeadLettered
	EventRangeQuarantined             = consume.EventRangeQuarantined
	EventSlowDispatch                 = consume.EventSlowDispatch
	EventQueuedRangeStale             = consume.EventQueuedRangeStale
	EventStoredOptionsClamped         = consume.EventStoredOptionsClamped
	EventGoRoutineEventsDropped       = metric.EventGoRoutineEventsDropped
	EventMeasurementsCannotBeExported = metric.EventMeasurementsCannotBeExported
	EventPartitionCreatedOnInsert     = produce.EventPartitionCreatedOnInsert
	EventPartitionNotCreatedAhead     = produce.EventPartitionNotCreatedAhead
	EventSlowProduce                  = produce.EventSlowProduce
	EventMessageAlreadyProduced       = schedule.EventMessageAlreadyProduced
	EventScheduleConfigReplaced       = schedule.EventScheduleConfigReplaced
	EventTargetKeepsNoSuccessRows     = schedule.EventTargetKeepsNoSuccessRows
	EventSystemManagerStopped         = system.EventSystemManagerStopped
	EventStreamConfigReplaced         = stream.EventStreamConfigReplaced
	EventInstanceLost                 = worker.EventInstanceLost
	EventManagerRowSuspended          = worker.EventManagerRowSuspended
	EventSlowTick                     = worker.EventSlowTick
	EventTickBackoffCurveExhausted    = worker.EventTickBackoffCurveExhausted
	EventWorkerConfigReplaced         = worker.EventWorkerConfigReplaced
)
