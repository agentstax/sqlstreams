package vacuum

import (
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
)

type VacuumConfig struct {
	// PollRate - the delay after each request, with scheduling jitter.
	// Default: 2m.
	PollRate time.Duration

	// VacuumTimeout - the maximum duration of a request, including retries.
	// Default: 1m.
	VacuumTimeout time.Duration

	// InstanceTTL is how long the claimed worker_instance row stays live
	// without a renewal -- past it the instance counts as dead and a
	// replacement can claim. The heartbeat renews at half this.
	// Default: 30s.
	InstanceTTL time.Duration

	// JitterFraction spreads vacuum ticks out of phase: each tick's delay is
	// poll_rate * (1 ± JitterFraction).
	// Default: 0.1. Must be < 1.
	JitterFraction float64

	VacuumRetry *common.RetryPolicy // failed-vacuum backoff curve. Default: common.NewDefaultRetryPolicy().
}

func (c *VacuumConfig) WithDefaults() *VacuumConfig {
	if c.PollRate == 0 {
		c.PollRate = 2 * time.Minute
	}
	if c.VacuumTimeout == 0 {
		c.VacuumTimeout = time.Minute
	}
	if c.InstanceTTL == 0 {
		c.InstanceTTL = 30 * time.Second
	}
	if c.JitterFraction == 0 {
		c.JitterFraction = 0.1
	}
	c.VacuumRetry = c.VacuumRetry.WithDefaults()
	return c
}

func (c *VacuumConfig) Validate() error {
	if c.PollRate <= 0 {
		return fmt.Errorf("PollRate must be > 0, got %v", c.PollRate)
	}
	if c.VacuumTimeout <= 0 {
		return fmt.Errorf("VacuumTimeout must be > 0, got %v", c.VacuumTimeout)
	}
	if c.InstanceTTL <= 0 {
		return fmt.Errorf("InstanceTTL must be > 0, got %v", c.InstanceTTL)
	}
	if c.JitterFraction < 0 || c.JitterFraction >= 1 {
		return fmt.Errorf("JitterFraction must be in [0, 1), got %v", c.JitterFraction)
	}
	if err := c.VacuumRetry.Validate(); err != nil {
		return fmt.Errorf("VacuumRetry: %w", err)
	}
	return nil
}
