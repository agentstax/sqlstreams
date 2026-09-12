package manager

import (
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
)

type ManagerConfig struct {
	// InstanceTTL is how long the claimed worker_instance row stays live
	// without a renewal -- past it the instance counts as dead and a
	// replacement can claim. The heartbeat renews at half this.
	// Default: 30s.
	InstanceTTL time.Duration

	// InstanceLogTTL retains worker instance snapshots after their recorded lease expiry.
	// Default: 24h.
	InstanceLogTTL time.Duration

	// JitterFraction scales discovery and declined-claim delays by 1 ± JitterFraction
	// to spread retries across replicas. Must be in [0, 1).
	// Default: 0.1.
	JitterFraction float64

	RefreshRetry *common.RetryPolicy // failed-refresh backoff curve. Default: common.NewDefaultRetryPolicy().

	// ClaimRetry backs off declined claims until a later manager tick retries them.
	// MaxRetries limits delay growth, not attempts; JitterFraction also applies.
	// Default: BaseDelay 1s, MaxDelay 30s; other fields use common.NewDefaultRetryPolicy().
	ClaimRetry *common.RetryPolicy
}

func (c *ManagerConfig) WithDefaults() *ManagerConfig {
	if c.InstanceTTL == 0 {
		c.InstanceTTL = 30 * time.Second
	}
	if c.InstanceLogTTL == 0 {
		c.InstanceLogTTL = 24 * time.Hour
	}
	if c.JitterFraction == 0 {
		c.JitterFraction = 0.1
	}
	c.RefreshRetry = c.RefreshRetry.WithDefaults()
	if c.ClaimRetry == nil {
		c.ClaimRetry = &common.RetryPolicy{BaseDelay: time.Second, MaxDelay: 30 * time.Second}
	}
	c.ClaimRetry.WithDefaults()
	return c
}

func (c *ManagerConfig) Validate() error {
	if c.InstanceTTL <= 0 {
		return fmt.Errorf("InstanceTTL must be > 0, got %v", c.InstanceTTL)
	}
	if c.InstanceLogTTL <= 0 {
		return fmt.Errorf("InstanceLogTTL must be > 0, got %v", c.InstanceLogTTL)
	}
	if c.JitterFraction < 0 || c.JitterFraction >= 1 {
		return fmt.Errorf("JitterFraction must be in [0, 1), got %v", c.JitterFraction)
	}
	if err := c.RefreshRetry.Validate(); err != nil {
		return fmt.Errorf("RefreshRetry: %w", err)
	}
	if err := c.ClaimRetry.Validate(); err != nil {
		return fmt.Errorf("ClaimRetry: %w", err)
	}
	return nil
}
