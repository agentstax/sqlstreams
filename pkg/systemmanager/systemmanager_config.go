package systemmanager

import (
	"fmt"

	"github.com/agentstax/vulkan/pkg/common"
)

type SystemManagerConfig struct {
	// JitterFraction spreads Run's between-life delays out of phase across
	// replicas: each delay is RunRetry's curve value * (1 ± JitterFraction).
	// Default: 0.1. Must be < 1.
	JitterFraction float64

	RunRetry *common.RetryPolicy // backoff between reconcile-loop lives after one ends on its own, unrelated to Retry above. Default: common.NewDefaultRetryPolicy().
}

func (c *SystemManagerConfig) WithDefaults() *SystemManagerConfig {
	if c.JitterFraction == 0 {
		c.JitterFraction = 0.1
	}
	c.RunRetry = c.RunRetry.WithDefaults()
	return c
}

// Validate runs after WithDefaults -- anything still out of range here was
// set by the caller, not left unset.
func (c *SystemManagerConfig) Validate() error {
	if c.JitterFraction < 0 || c.JitterFraction >= 1 {
		return fmt.Errorf("JitterFraction must be in [0, 1), got %v", c.JitterFraction)
	}
	if err := c.RunRetry.Validate(); err != nil {
		return fmt.Errorf("RunRetry: %w", err)
	}
	return nil
}
