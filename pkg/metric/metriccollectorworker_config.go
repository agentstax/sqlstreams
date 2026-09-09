package metric

import (
	"fmt"
	"time"
)

// MetricCollectorWorkerConfig declares the metrics_collector worker row:
// how often the collector measures the fleet and produces to
// __system.metrics.
type MetricCollectorWorkerConfig struct {
	// PollRate - how often one collection pass runs.
	// Default: 30s.
	PollRate time.Duration
}

func (c *MetricCollectorWorkerConfig) WithDefaults() *MetricCollectorWorkerConfig {
	if c.PollRate == 0 {
		c.PollRate = 30 * time.Second
	}
	return c
}

func (c *MetricCollectorWorkerConfig) Validate() error {
	if c.PollRate <= 0 {
		return fmt.Errorf("PollRate must be > 0, got %v", c.PollRate)
	}
	return nil
}
