package stream

import (
	"fmt"
	"time"
)

// JanitorConfig declares retention cleanup settings.
// A new stream's janitor starts active.
type JanitorConfig struct {
	// PollRate - the delay between cleanup passes, with scheduling jitter.
	// Default: 5s.
	PollRate time.Duration

	// SweepBatchSize - the maximum rows deleted per transaction.
	// Default: 1000.
	SweepBatchSize int

	// CleanupTimeout - the maximum duration of each cleanup operation, including retries.
	// Default: 5s.
	CleanupTimeout time.Duration

	// PartialSweepGracePeriod - extra retention before partial message cleanup.
	// Default: 0 (no extra retention).
	PartialSweepGracePeriod time.Duration
}

func (c *JanitorConfig) WithDefaults() *JanitorConfig {
	if c.PollRate == 0 {
		c.PollRate = 5 * time.Second
	}
	if c.SweepBatchSize == 0 {
		c.SweepBatchSize = 1000
	}
	if c.CleanupTimeout == 0 {
		c.CleanupTimeout = 5 * time.Second
	}
	return c
}

func (c *JanitorConfig) Validate() error {
	if c.PollRate <= 0 {
		return fmt.Errorf("PollRate must be > 0, got %v", c.PollRate)
	}
	if c.SweepBatchSize <= 0 {
		return fmt.Errorf("SweepBatchSize must be > 0, got %d", c.SweepBatchSize)
	}
	if c.CleanupTimeout <= 0 {
		return fmt.Errorf("CleanupTimeout must be > 0, got %v", c.CleanupTimeout)
	}
	if c.PartialSweepGracePeriod < 0 {
		return fmt.Errorf("PartialSweepGracePeriod must be >= 0, got %v", c.PartialSweepGracePeriod)
	}
	return nil
}
