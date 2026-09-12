package collector

import (
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
)

type MetricCollectorConfig struct {
	// PollRate - the collection interval declared on the worker row.
	// Default: 30s.
	PollRate time.Duration

	// InstanceTTL is how long the claimed worker_instance row stays live
	// without a renewal -- past it the instance counts as dead and a
	// replacement can claim. The heartbeat renews at half this.
	// Default: 30s.
	InstanceTTL time.Duration

	// JitterFraction spreads collection ticks out of phase: each tick's delay
	// is poll_rate * (1 ± JitterFraction).
	// Default: 0.1. Must be < 1.
	JitterFraction float64

	// StreamConcurrency caps how many streams one collection pass snapshots
	// and produces at once. The collector shares its connection pool with
	// the process embedding it, so the cap is what keeps a large stream
	// count from crowding out that process's own traffic.
	// Default: 4.
	StreamConcurrency int

	CollectRetry *common.RetryPolicy // failed-collection backoff curve. Default: common.NewDefaultRetryPolicy().
}

func (c *MetricCollectorConfig) WithDefaults() *MetricCollectorConfig {
	if c.PollRate == 0 {
		c.PollRate = 30 * time.Second
	}
	if c.InstanceTTL == 0 {
		c.InstanceTTL = 30 * time.Second
	}
	if c.JitterFraction == 0 {
		c.JitterFraction = 0.1
	}
	if c.StreamConcurrency == 0 {
		c.StreamConcurrency = 4
	}
	c.CollectRetry = c.CollectRetry.WithDefaults()
	return c
}

func (c *MetricCollectorConfig) Validate() error {
	if c.PollRate <= 0 {
		return fmt.Errorf("PollRate must be > 0, got %v", c.PollRate)
	}
	if c.InstanceTTL <= 0 {
		return fmt.Errorf("InstanceTTL must be > 0, got %v", c.InstanceTTL)
	}
	if c.JitterFraction < 0 || c.JitterFraction >= 1 {
		return fmt.Errorf("JitterFraction must be in [0, 1), got %v", c.JitterFraction)
	}
	if c.StreamConcurrency < 1 {
		return fmt.Errorf("StreamConcurrency must be >= 1, got %d", c.StreamConcurrency)
	}
	if err := c.CollectRetry.Validate(); err != nil {
		return fmt.Errorf("CollectRetry: %w", err)
	}
	return nil
}
