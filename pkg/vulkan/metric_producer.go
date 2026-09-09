package vulkan

import "context"

type MetricProducerHandle struct {
	client *Client
}

// Producer selects the system metrics topic for custom measurements. No I/O.
func (s *SystemMetricsHandle) Producer() *MetricProducerHandle {
	return &MetricProducerHandle{client: s.client}
}

// Register resolves the system metrics topic using the client's existing
// datastore. Register the system first; cfg may be nil or sparse.
func (p *MetricProducerHandle) Register(ctx context.Context, cfg *ProducerConfig) (*MetricProducerInstance, error) {
	return p.client.producer.RegisterMetrics(ctx, cfg)
}
