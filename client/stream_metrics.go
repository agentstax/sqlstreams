package sqlstreams

import (
	"context"

	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
	"github.com/agentstax/sqlstreams/pkg/metric"
)

// StreamMetricsHandle names one stream's metrics resource, holding no database
// row.
type StreamMetricsHandle struct {
	streamName string
	client     *Client
}

// Metrics returns the stream's metrics handle. It performs no I/O.
func (t *StreamHandle[Message]) Metrics() *StreamMetricsHandle {
	return &StreamMetricsHandle{streamName: t.name, client: t.client}
}

// Definitions returns the stream-scoped SQLStreams metric definitions ordered by SQL
// code. It performs no I/O.
func (t *StreamMetricsHandle) Definitions() []MetricDefinition {
	return metric.Definitions(diagnostic.MetricScopeStream)
}

// Snapshot computes the stream's live metrics from its source tables.
func (t *StreamMetricsHandle) Snapshot(ctx context.Context) (*StreamSnapshot, error) {
	return t.client.admin.StreamMetrics(ctx, t.streamName)
}

// Compacted selects the stream's compacted-state series.
func (t *StreamMetricsHandle) Compacted() *MetricHandle {
	return t.metric(metric.MetricStreamCompacted)
}

// Partitions selects the stream's message-log partition-count series.
func (t *StreamMetricsHandle) Partitions() *MetricHandle {
	return t.metric(metric.MetricStreamPartitions)
}

// UnclaimedWorkers selects the unclaimed workers owned by the stream and its groups.
func (t *StreamMetricsHandle) UnclaimedWorkers() *MetricHandle {
	return t.metric(metric.MetricStreamUnclaimedWorkers)
}

func (t *StreamMetricsHandle) metric(declared *diagnostic.DiagnosticMetric) *MetricHandle {
	attributes := map[string]string{"stream": t.streamName}
	return newMetricHandle(t.client, declared, declared.Name, attributes)
}
