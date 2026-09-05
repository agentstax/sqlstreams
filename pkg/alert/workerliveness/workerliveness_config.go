package workerliveness

import (
	"fmt"
	"time"
)

type WorkerLivenessConfig struct {
	// InstanceTTL - how long the claimed worker_instance row stays live
	// between heartbeats.
	// Default: 30s.
	InstanceTTL time.Duration

	// RepeatInterval - how long an active alert stays quiet before it
	// repeats as a reminder.
	// Default: 4h.
	RepeatInterval time.Duration
}

func (c *WorkerLivenessConfig) WithDefaults() *WorkerLivenessConfig {
	if c.InstanceTTL == 0 {
		c.InstanceTTL = 30 * time.Second
	}
	if c.RepeatInterval == 0 {
		c.RepeatInterval = 4 * time.Hour
	}
	return c
}

// Validate runs after WithDefaults -- anything still out of range here was
// set by the caller, not left unset.
func (c *WorkerLivenessConfig) Validate() error {
	if c.InstanceTTL <= 0 {
		return fmt.Errorf("InstanceTTL must be > 0, got %v", c.InstanceTTL)
	}
	if c.RepeatInterval <= 0 {
		return fmt.Errorf("RepeatInterval must be > 0, got %v", c.RepeatInterval)
	}
	return nil
}
