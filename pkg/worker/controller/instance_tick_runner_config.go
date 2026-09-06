package controller

import (
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
)

type InstanceTickRunnerConfig struct {
	// InstanceTTL is how long the claimed worker_instance row stays live
	// without a renewal -- past it the instance counts as dead and a
	// replacement can claim. The heartbeat renews at half this.
	// Default: 30s.
	InstanceTTL time.Duration

	// JitterFraction spreads ticks out of phase: each tick's delay is
	// poll_rate * (1 ± JitterFraction).
	// Default: 0.1. Must be < 1.
	JitterFraction float64

	TickRetry *common.RetryPolicy // failed-tick backoff curve. Default: common.NewDefaultRetryPolicy().
}

func (c *InstanceTickRunnerConfig) WithDefaults() *InstanceTickRunnerConfig {
	if c.InstanceTTL == 0 {
		c.InstanceTTL = 30 * time.Second
	}
	if c.JitterFraction == 0 {
		c.JitterFraction = 0.1
	}
	c.TickRetry = c.TickRetry.WithDefaults()
	return c
}

func (c *InstanceTickRunnerConfig) Validate() error {
	if c.InstanceTTL <= 0 {
		return fmt.Errorf("InstanceTTL must be > 0, got %v", c.InstanceTTL)
	}
	if c.JitterFraction < 0 || c.JitterFraction >= 1 {
		return fmt.Errorf("JitterFraction must be in [0, 1), got %v", c.JitterFraction)
	}
	if err := c.TickRetry.Validate(); err != nil {
		return fmt.Errorf("TickRetry: %w", err)
	}
	return nil
}
