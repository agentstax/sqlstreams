package producer

import (
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
)

type ScheduleProducerConfig struct {
	// InstanceTTL is how long the claimed worker_instance row stays live
	// without a renewal -- past it the instance counts as dead and a
	// replacement can claim. The heartbeat renews at half this.
	// Default: 30s.
	InstanceTTL time.Duration

	// JitterFraction spreads scan ticks out of phase: each tick's delay is
	// poll_rate * (1 ± JitterFraction).
	// Default: 0.1. Must be < 1.
	JitterFraction float64

	ScanRetry *common.RetryPolicy // failed-scan backoff curve. Default: common.NewDefaultRetryPolicy().
}

func (c *ScheduleProducerConfig) WithDefaults() *ScheduleProducerConfig {
	if c.InstanceTTL == 0 {
		c.InstanceTTL = 30 * time.Second
	}
	if c.JitterFraction == 0 {
		c.JitterFraction = 0.1
	}
	c.ScanRetry = c.ScanRetry.WithDefaults()
	return c
}

func (c *ScheduleProducerConfig) Validate() error {
	if c.InstanceTTL <= 0 {
		return fmt.Errorf("InstanceTTL must be > 0, got %v", c.InstanceTTL)
	}
	if c.JitterFraction < 0 || c.JitterFraction >= 1 {
		return fmt.Errorf("JitterFraction must be in [0, 1), got %v", c.JitterFraction)
	}
	if err := c.ScanRetry.Validate(); err != nil {
		return fmt.Errorf("ScanRetry: %w", err)
	}
	return nil
}
