package manager

import (
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
)

type ManagerConfig struct {
	// InstanceTTL is how long the claimed worker_instance row stays live
	// without a renewal -- past it the instance counts as dead and a
	// replacement can claim. The heartbeat renews at half this.
	// Default: 30s.
	InstanceTTL time.Duration

	// JitterFraction spreads discovery ticks out of phase across manager
	// replicas: each tick's delay is the row's poll_rate * (1 ± JitterFraction).
	// Default: 0.1. Must be < 1.
	JitterFraction float64

	RefreshRetry *common.RetryPolicy // failed-refresh backoff curve. Default: common.NewDefaultRetryPolicy().
}

func (c *ManagerConfig) WithDefaults() *ManagerConfig {
	if c.InstanceTTL == 0 {
		c.InstanceTTL = 30 * time.Second
	}
	if c.JitterFraction == 0 {
		c.JitterFraction = 0.1
	}
	c.RefreshRetry = c.RefreshRetry.WithDefaults()
	return c
}

func (c *ManagerConfig) Validate() error {
	if c.InstanceTTL <= 0 {
		return fmt.Errorf("InstanceTTL must be > 0, got %v", c.InstanceTTL)
	}
	if c.JitterFraction < 0 || c.JitterFraction >= 1 {
		return fmt.Errorf("JitterFraction must be in [0, 1), got %v", c.JitterFraction)
	}
	if err := c.RefreshRetry.Validate(); err != nil {
		return fmt.Errorf("RefreshRetry: %w", err)
	}
	return nil
}
