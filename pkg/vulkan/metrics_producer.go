package vulkan

import "context"

type MetricsProducerHandle struct {
	client *Client
}

// Producer selects the system metrics topic for custom measurements. No I/O.
func (s *SystemMetricsHandle) Producer() *MetricsProducerHandle {
	return &MetricsProducerHandle{client: s.client}
}

// Register resolves the system metrics topic using the client's existing
// datastore. Register the system first; cfg may be nil or sparse.
func (p *MetricsProducerHandle) Register(ctx context.Context, cfg *ProducerConfig) (*MetricsProducerInstance, error) {
	return p.client.producer.RegisterMetrics(ctx, cfg)
}
