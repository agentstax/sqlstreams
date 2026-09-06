package janitor

import (
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
)

type JanitorConfig struct {
	// CleanupTimeout limits each cleanup operation, including its queries and retries.
	// Default: 5s.
	CleanupTimeout time.Duration

	// InstanceTTL is how long the claimed worker_instance row stays live
	// without a renewal -- past it the instance counts as dead and a
	// replacement can claim. The heartbeat renews at half this.
	// Default: 30s.
	InstanceTTL time.Duration

	// JitterFraction spreads sweep ticks out of phase: each tick's delay is
	// poll_rate * (1 ± JitterFraction).
	// Default: 0.1. Must be < 1.
	JitterFraction float64

	SweepRetry *common.RetryPolicy // failed-sweep backoff curve. Default: common.NewDefaultRetryPolicy().
}

func (c *JanitorConfig) WithDefaults() *JanitorConfig {
	if c.CleanupTimeout == 0 {
		c.CleanupTimeout = 5 * time.Second
	}
	if c.InstanceTTL == 0 {
		c.InstanceTTL = 30 * time.Second
	}
	if c.JitterFraction == 0 {
		c.JitterFraction = 0.1
	}
	c.SweepRetry = c.SweepRetry.WithDefaults()
	return c
}

func (c *JanitorConfig) Validate() error {
	if c.CleanupTimeout <= 0 {
		return fmt.Errorf("CleanupTimeout must be > 0, got %v", c.CleanupTimeout)
	}
	if c.InstanceTTL <= 0 {
		return fmt.Errorf("InstanceTTL must be > 0, got %v", c.InstanceTTL)
	}
	if c.JitterFraction < 0 || c.JitterFraction >= 1 {
		return fmt.Errorf("JitterFraction must be in [0, 1), got %v", c.JitterFraction)
	}
	if err := c.SweepRetry.Validate(); err != nil {
		return fmt.Errorf("SweepRetry: %w", err)
	}
	return nil
}
