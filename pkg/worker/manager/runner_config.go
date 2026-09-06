package manager

import (
	"fmt"
	"time"
)

type RunnerConfig struct {
	// RetryDelay is the pause between claim attempt. Keep it >= the provisioner's
	// InstanceTTL -- anything shorter re-claims a row whose live instances
	// haven't expired yet.
	// Default: 30s, matching ManagerConfig.InstanceTTL's default.
	RetryDelay time.Duration

	// JitterFraction spreads retries out of phase across replicas that lost
	// their claims together: each delay is RetryDelay * (1 ± JitterFraction).
	// Default: 0.1. Must be < 1.
	JitterFraction float64
}

func (c *RunnerConfig) WithDefaults() *RunnerConfig {
	if c.RetryDelay == 0 {
		c.RetryDelay = 30 * time.Second
	}
	if c.JitterFraction == 0 {
		c.JitterFraction = 0.1
	}
	return c
}

func (c *RunnerConfig) Validate() error {
	if c.RetryDelay <= 0 {
		return fmt.Errorf("RetryDelay must be > 0, got %v", c.RetryDelay)
	}
	if c.JitterFraction < 0 || c.JitterFraction >= 1 {
		return fmt.Errorf("JitterFraction must be in [0, 1), got %v", c.JitterFraction)
	}
	return nil
}
