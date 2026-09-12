package sqlstreams

import (
	"context"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/diagnostic"
)

// StreamAlertsHandle names one stream's alerts resource, holding no database
// row.
type StreamAlertsHandle struct {
	streamName string
	client     *Client
}

// Alerts returns the stream's alerts handle. It performs no I/O.
func (t *StreamHandle[Message]) Alerts() *StreamAlertsHandle {
	return &StreamAlertsHandle{streamName: t.name, client: t.client}
}

// Definitions returns the stream-scoped SQLStreams alert definitions ordered by
// SQL code. It performs no I/O.
func (t *StreamAlertsHandle) Definitions() []AlertDefinition {
	return alert.Definitions(diagnostic.MetricScopeStream)
}

// Latest returns the current alert per name owned by the stream, active or
// resolved, ordered by message key. Returns ErrStreamNotFound when the stream
// is not registered.
func (t *StreamAlertsHandle) Latest(ctx context.Context) ([]*Alert, error) {
	owner, err := t.client.admin.StreamOwner(ctx, t.streamName)
	if err != nil {
		return nil, err
	}

	stored, err := t.client.admin.ListAlerts(ctx)
	if err != nil {
		return nil, err
	}
	alerts := make([]*Alert, 0, len(stored))
	for _, head := range stored {
		if head.Message.Owner.Kind() == common.OwnerStream && head.Message.Owner.StreamId == owner.StreamId {
			alerts = append(alerts, head.Message)
		}
	}
	return alerts, nil
}

// Alert names one alert owned by the stream. It performs no I/O. Built-in
// selectors are preferred when one is available.
func (t *StreamAlertsHandle) Alert(name string) *AlertHandle {
	return newAlertHandle(t.client, name, t.streamName, "")
}

// PartitionCount selects the stream's partition_count alert.
func (t *StreamAlertsHandle) PartitionCount() *AlertHandle {
	return t.Alert(alert.AlertPartitionCount.Name)
}

// CompactionReadCost selects the stream's compaction_read_cost alert.
func (t *StreamAlertsHandle) CompactionReadCost() *AlertHandle {
	return t.Alert(alert.AlertCompactionReadCost.Name)
}

// WorkerLiveness selects the stream's worker_liveness alert.
func (t *StreamAlertsHandle) WorkerLiveness() *AlertHandle {
	return t.Alert(alert.AlertWorkerLiveness.Name)
}
