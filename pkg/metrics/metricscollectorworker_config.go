package metrics

import (
	"fmt"
	"time"
)

// MetricsCollectorWorkerConfig declares the metrics_collector worker row:
// how often the collector measures the fleet and produces to
// __system.metrics.
type MetricsCollectorWorkerConfig struct {
	// PollRate - how often one collection pass runs.
	// Default: 30s.
	PollRate time.Duration
}

func (c *MetricsCollectorWorkerConfig) WithDefaults() *MetricsCollectorWorkerConfig {
	if c.PollRate == 0 {
		c.PollRate = 30 * time.Second
	}
	return c
}

// Validate runs after WithDefaults -- anything still out of range here was
// set by the caller, not left unset.
func (c *MetricsCollectorWorkerConfig) Validate() error {
	if c.PollRate <= 0 {
		return fmt.Errorf("PollRate must be > 0, got %v", c.PollRate)
	}
	return nil
}
