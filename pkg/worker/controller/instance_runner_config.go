package controller

import (
	"fmt"
	"time"
)

type InstanceRunnerConfig struct {
	// InstanceTTL is how long the claimed worker_instance row stays live
	// without a renewal -- past it the instance counts as dead and a
	// replacement can claim. The heartbeat renews at half this.
	// Default: 30s.
	InstanceTTL time.Duration

	// JitterFraction spreads renewals out of phase across instances claimed
	// together: each renewal waits InstanceTTL/2 * (1 ± JitterFraction).
	// Default: 0.1. Must be < 1.
	JitterFraction float64
}

func (c *InstanceRunnerConfig) WithDefaults() *InstanceRunnerConfig {
	if c.InstanceTTL == 0 {
		c.InstanceTTL = 30 * time.Second
	}
	if c.JitterFraction == 0 {
		c.JitterFraction = 0.1
	}
	return c
}

func (c *InstanceRunnerConfig) Validate() error {
	if c.InstanceTTL <= 0 {
		return fmt.Errorf("InstanceTTL must be > 0, got %v", c.InstanceTTL)
	}
	if c.JitterFraction < 0 || c.JitterFraction >= 1 {
		return fmt.Errorf("JitterFraction must be in [0, 1), got %v", c.JitterFraction)
	}
	return nil
}
