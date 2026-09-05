package vulkan

import (
	"context"

	"github.com/agentstax/vulkan/pkg/common/diagnostic"
	"github.com/agentstax/vulkan/pkg/metrics"
)

// ConsumerMetricsHandle names one consumer group's metrics resource, holding no
// database row.
type ConsumerMetricsHandle struct {
	topicName string
	groupName string
	client    *Client
}

// Metrics returns the consumer group's metrics handle. It performs no I/O.
func (h *ConsumerHandle[Message]) Metrics() *ConsumerMetricsHandle {
	return &ConsumerMetricsHandle{topicName: h.topicName, groupName: h.name, client: h.client}
}

// Definitions returns the consumer-group-scoped Vulkan metric definitions
// ordered by VK code. It performs no I/O.
func (h *ConsumerMetricsHandle) Definitions() []MetricDefinition {
	return metrics.Definitions(diagnostic.MetricScopeConsumerGroup)
}

// Snapshot computes the consumer group's live metrics from its source tables.
func (h *ConsumerMetricsHandle) Snapshot(ctx context.Context) (*ConsumerGroupSnapshot, error) {
	return h.client.admin.GroupMetrics(ctx, h.topicName, h.groupName)
}

// CursorHead selects the group's topic-head series.
func (h *ConsumerMetricsHandle) CursorHead() *MetricHandle {
	return h.metric(metrics.MetricCursorHead)
}

// CursorClaimed selects the group's claimed-cursor series.
func (h *ConsumerMetricsHandle) CursorClaimed() *MetricHandle {
	return h.metric(metrics.MetricCursorClaimed)
}

// CursorCommitted selects the group's committed-cursor series.
func (h *ConsumerMetricsHandle) CursorCommitted() *MetricHandle {
	return h.metric(metrics.MetricCursorCommitted)
}

// CursorBacklog selects the group's cursor-backlog series.
func (h *ConsumerMetricsHandle) CursorBacklog() *MetricHandle {
	return h.metric(metrics.MetricCursorBacklog)
}

// CursorInflight selects the group's cursor-inflight series.
func (h *ConsumerMetricsHandle) CursorInflight() *MetricHandle {
	return h.metric(metrics.MetricCursorInflight)
}

// ReadyExceptions selects the group's ready-exception series.
func (h *ConsumerMetricsHandle) ReadyExceptions() *MetricHandle {
	return h.metric(metrics.MetricReadyExceptions)
}

// InflightExceptions selects the group's inflight-exception series.
func (h *ConsumerMetricsHandle) InflightExceptions() *MetricHandle {
	return h.metric(metrics.MetricInflightExceptions)
}

// DeferredExceptions selects the group's deferred-exception series.
func (h *ConsumerMetricsHandle) DeferredExceptions() *MetricHandle {
	return h.metric(metrics.MetricDeferredExceptions)
}

// DeadExceptions selects the group's dead-exception series.
func (h *ConsumerMetricsHandle) DeadExceptions() *MetricHandle {
	return h.metric(metrics.MetricDeadExceptions)
}

// OldestUnresolvedAge selects the group's oldest-unresolved-exception-age
// series.
func (h *ConsumerMetricsHandle) OldestUnresolvedAge() *MetricHandle {
	return h.metric(metrics.MetricOldestUnresolvedAge)
}

// OpenLeases selects the group's open-lease series.
func (h *ConsumerMetricsHandle) OpenLeases() *MetricHandle {
	return h.metric(metrics.MetricOpenLeases)
}

// AbandonedRoutinesOutstanding selects the group's outstanding-abandoned-
// routines series.
func (h *ConsumerMetricsHandle) AbandonedRoutinesOutstanding() *MetricHandle {
	return h.metric(metrics.MetricAbandonedOutstanding)
}

// AbandonedRoutinesTotal selects the group's total-abandoned-routines series.
func (h *ConsumerMetricsHandle) AbandonedRoutinesTotal() *MetricHandle {
	return h.metric(metrics.MetricAbandonedTotal)
}

// AbandonedRoutinesSelfClearLatencyAverage selects the group's average
// abandoned-routine self-clear-latency series.
func (h *ConsumerMetricsHandle) AbandonedRoutinesSelfClearLatencyAverage() *MetricHandle {
	return h.metric(metrics.MetricAbandonedSelfClearLatencyAvg)
}

func (h *ConsumerMetricsHandle) metric(declared *diagnostic.DiagnosticMetric) *MetricHandle {
	attributes := map[string]string{"topic": h.topicName, "group": h.groupName}
	return newMetricHandle(h.client, declared, declared.Name, attributes)
}
