package sqlstreams

// Every declared log event, under its own name, for callers that filter
// on a code.

import (
	"github.com/allegedlyreliable/sqlstreams/pkg/alert"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
	"github.com/allegedlyreliable/sqlstreams/pkg/produce"
	"github.com/allegedlyreliable/sqlstreams/pkg/schedule"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/allegedlyreliable/sqlstreams/pkg/system"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
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
