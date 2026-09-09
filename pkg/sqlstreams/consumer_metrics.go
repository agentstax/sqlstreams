package sqlstreams

import (
	"context"

	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
	"github.com/agentstax/sqlstreams/pkg/metric"
)

// ConsumerMetricsHandle names one consumer group's metrics resource, holding no
// database row.
type ConsumerMetricsHandle struct {
	streamName string
	groupName  string
	client     *Client
}

// Metrics returns the consumer group's metrics handle. It performs no I/O.
func (h *ConsumerHandle[Message]) Metrics() *ConsumerMetricsHandle {
	return &ConsumerMetricsHandle{streamName: h.streamName, groupName: h.name, client: h.client}
}

// Definitions returns the consumer-group-scoped SQLStreams metric definitions
// ordered by SQL code. It performs no I/O.
func (h *ConsumerMetricsHandle) Definitions() []MetricDefinition {
	return metric.Definitions(diagnostic.MetricScopeConsumerGroup)
}

// Snapshot computes the consumer group's live metrics from its source tables.
func (h *ConsumerMetricsHandle) Snapshot(ctx context.Context) (*ConsumerGroupSnapshot, error) {
	return h.client.admin.ConsumerGroupMetrics(ctx, h.streamName, h.groupName)
}

// CursorHead selects the group's stream-head series.
func (h *ConsumerMetricsHandle) CursorHead() *MetricHandle {
	return h.metric(metric.MetricCursorHead)
}

// CursorClaimed selects the group's claimed-cursor series.
func (h *ConsumerMetricsHandle) CursorClaimed() *MetricHandle {
	return h.metric(metric.MetricCursorClaimed)
}

// CursorCommitted selects the group's committed-cursor series.
func (h *ConsumerMetricsHandle) CursorCommitted() *MetricHandle {
	return h.metric(metric.MetricCursorCommitted)
}

// CursorBacklog selects the group's cursor-backlog series.
func (h *ConsumerMetricsHandle) CursorBacklog() *MetricHandle {
	return h.metric(metric.MetricCursorBacklog)
}

// CursorInflight selects the group's cursor-inflight series.
func (h *ConsumerMetricsHandle) CursorInflight() *MetricHandle {
	return h.metric(metric.MetricCursorInflight)
}

// ReadyExceptions selects the group's ready-exception series.
func (h *ConsumerMetricsHandle) ReadyExceptions() *MetricHandle {
	return h.metric(metric.MetricReadyExceptions)
}

// InflightExceptions selects the group's inflight-exception series.
func (h *ConsumerMetricsHandle) InflightExceptions() *MetricHandle {
	return h.metric(metric.MetricInflightExceptions)
}

// DeferredExceptions selects the group's deferred-exception series.
func (h *ConsumerMetricsHandle) DeferredExceptions() *MetricHandle {
	return h.metric(metric.MetricDeferredExceptions)
}

// DeadExceptions selects the group's dead-exception series.
func (h *ConsumerMetricsHandle) DeadExceptions() *MetricHandle {
	return h.metric(metric.MetricDeadExceptions)
}

// OldestUnresolvedAge selects the group's oldest-unresolved-exception-age
// series.
func (h *ConsumerMetricsHandle) OldestUnresolvedAge() *MetricHandle {
	return h.metric(metric.MetricOldestUnresolvedAge)
}

// OpenLeases selects the group's open-lease series.
func (h *ConsumerMetricsHandle) OpenLeases() *MetricHandle {
	return h.metric(metric.MetricOpenLeases)
}

// AbandonedRoutinesOutstanding selects the group's outstanding-abandoned-
// routines series.
func (h *ConsumerMetricsHandle) AbandonedRoutinesOutstanding() *MetricHandle {
	return h.metric(metric.MetricAbandonedOutstanding)
}

// AbandonedRoutinesTotal selects the group's total-abandoned-routines series.
func (h *ConsumerMetricsHandle) AbandonedRoutinesTotal() *MetricHandle {
	return h.metric(metric.MetricAbandonedTotal)
}

// AbandonedRoutinesSelfClearLatencyAverage selects the group's average
// abandoned-routine self-clear-latency series.
func (h *ConsumerMetricsHandle) AbandonedRoutinesSelfClearLatencyAverage() *MetricHandle {
	return h.metric(metric.MetricAbandonedSelfClearLatencyAvg)
}

func (h *ConsumerMetricsHandle) metric(declared *diagnostic.DiagnosticMetric) *MetricHandle {
	attributes := map[string]string{"stream": h.streamName, "group": h.groupName}
	return newMetricHandle(h.client, declared, declared.Name, attributes)
}
