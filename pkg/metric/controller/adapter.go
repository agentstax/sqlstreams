package controller

import (
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/metric/controller/datastore"
)

// overdueThreshold: how long a schedule may sit due and unproduced before it
// counts as overdue.
const overdueThreshold = 10 * time.Minute

func toMeasurementHistory(current time.Time, messages []*common.StoredMessage[metric.Measurement]) *metric.MeasurementHistory {
	return &metric.MeasurementHistory{EvaluatedAt: current, Messages: messages}
}

func toOwner(systemId int64, streamId int64, consumerGroupId int64, streamName string, groupName string) (*common.Owner, error) {
	switch {
	case consumerGroupId > 0:
		return common.NewConsumerGroupOwner(systemId, streamId, consumerGroupId, groupName)
	case streamId > 0:
		return common.NewStreamOwner(systemId, streamId, streamName)
	default:
		return common.NewSystemOwner(systemId)
	}
}

func toWorkerSnapshot(data datastore.WorkerSnapshotRow) (metric.WorkerSnapshot, error) {
	owner, err := toOwner(data.SystemId, data.StreamId, data.ConsumerGroupId, data.StreamName, data.GroupName)
	if err != nil {
		return metric.WorkerSnapshot{}, err
	}

	snapshot := metric.WorkerSnapshot{
		Owner:           owner,
		Name:            data.Name,
		Status:          classifyWorker(data.TargetInstances, data.LiveInstances),
		TargetInstances: data.TargetInstances,
		LiveInstances:   data.LiveInstances,
		Attempts:        data.MaxAttempts,
	}
	if data.LiveInstances == 0 && data.UnclaimedForSecs > 0 {
		snapshot.UnclaimedFor = time.Duration(data.UnclaimedForSecs * float64(time.Second))
	}
	return snapshot, nil
}

func classifyWorker(targetInstances int, liveInstances int) metric.WorkerStatus {
	switch {
	case targetInstances == 0:
		return metric.WorkerSuspended
	case liveInstances > 0:
		return metric.WorkerClaimed
	default:
		return metric.WorkerUnclaimed
	}
}

func toScheduleSnapshot(data datastore.ScheduleSnapshotRow) (metric.ScheduleSnapshot, error) {
	owner, err := common.NewSystemOwner(data.SystemId)
	if err != nil {
		return metric.ScheduleSnapshot{}, err
	}

	snapshot := metric.ScheduleSnapshot{
		Owner:           owner,
		Name:            data.Name,
		Stream:          data.StreamName,
		Expression:      data.Expression,
		Suspended:       data.Suspended,
		NextScheduledAt: data.NextScheduledAt,
		DueFor:          time.Duration(data.DueForSecs * float64(time.Second)),
	}
	if data.LastScheduledAt != nil {
		snapshot.LastScheduledAt = *data.LastScheduledAt
	}

	// a suspended row's next_scheduled_at goes stale on purpose --
	// unsuspending recomputes it, so staleness is never overdue
	snapshot.Overdue = !snapshot.Suspended && snapshot.DueFor > overdueThreshold
	return snapshot, nil
}

func toConsumerGroupSnapshot(consumerGroup string, data *datastore.ConsumerGroupSnapshotRow, abandonedRoutines metric.AbandonedRoutineSnapshot) *metric.ConsumerGroupSnapshot {
	snapshot := &metric.ConsumerGroupSnapshot{
		ConsumerGroup: consumerGroup,
		Cursor: metric.CursorSnapshot{
			Head:      data.Head,
			Claimed:   data.Claimed,
			Committed: data.Committed,
			Backlog:   data.Head - data.Committed,
			Inflight:  data.Claimed - data.Committed,
		},
		Exceptions: metric.ExceptionSnapshot{
			Ready:    data.ReadyExceptions,
			Inflight: data.InflightExceptions,
			Deferred: data.DeferredExceptions,
			Dead:     data.DeadExceptions,
		},
		OpenLeases:        data.OpenLeases,
		AbandonedRoutines: abandonedRoutines,
	}
	if data.OldestUnresolvedAt != nil {
		snapshot.Exceptions.OldestUnresolvedAge = time.Since(*data.OldestUnresolvedAt)
	}
	return snapshot
}

func toStreamSnapshot(streamId int64, data *datastore.StreamSnapshotRow, groups []metric.ConsumerGroupSnapshot) *metric.StreamSnapshot {
	return &metric.StreamSnapshot{
		StreamId:                          streamId,
		Partitions:                        data.Partitions,
		Compacted:                         data.Compacted,
		CompactionRowsWithoutHead:         data.CompactionRowsWithoutHead,
		OldestCompactionRowWithoutHeadAge: time.Duration(data.OldestCompactionRowWithoutHeadSecs * float64(time.Second)),
		Groups:                            groups,
	}
}

func toAbandonedRoutineSnapshot(abandoned []datastore.EventTimestampRow, cleared []datastore.EventTimestampRow) *metric.AbandonedRoutineSnapshot {
	// eventKey is the (message, attempt) identity an abandoned event and its
	// matching cleared event share -- streamId/group are already fixed by the
	// routing key both reads filter on, so they're not part of the key.
	type eventKey struct {
		MessageId int64
		Attempt   int
	}

	clearedAt := make(map[eventKey]time.Time, len(cleared))
	for _, event := range cleared {
		clearedAt[eventKey{MessageId: event.MessageId, Attempt: event.Attempt}] = event.At
	}

	var snapshot metric.AbandonedRoutineSnapshot
	var latencySum time.Duration
	var matched int64
	for _, event := range abandoned {
		snapshot.Total++
		at, ok := clearedAt[eventKey{MessageId: event.MessageId, Attempt: event.Attempt}]
		if !ok {
			snapshot.Outstanding++
			continue
		}
		latencySum += at.Sub(event.At)
		matched++
	}
	if matched > 0 {
		snapshot.SelfClearLatencyAvg = latencySum / time.Duration(matched)
	}
	return &snapshot
}

func toStreamSchemaVersionSnapshot(count *datastore.SchemaVersionCountRow, lags []datastore.ConsumerGroupSchemaVersionLagRow) metric.StreamSchemaVersionSnapshot {
	groups := make([]metric.ConsumerGroupSchemaVersionLag, 0, len(lags))
	for _, lag := range lags {
		groups = append(groups, metric.ConsumerGroupSchemaVersionLag{
			ConsumerGroup:        lag.ConsumerGroup,
			Unconsumed:           lag.Unconsumed,
			UnresolvedExceptions: lag.UnresolvedExceptions,
		})
	}
	return metric.StreamSchemaVersionSnapshot{
		Version:         int(count.SchemaVersion),
		Messages:        count.Messages,
		CompactionHeads: count.CompactionHeads,
		Groups:          groups,
	}
}
