package stream

import (
	"fmt"
	"time"
)

// VacuumConfig declares VACUUM (ANALYZE) settings for the stream's idempotency-key table.
// A new stream's vacuum starts suspended.
type VacuumConfig struct {
	// PollRate - the delay after each request, with scheduling jitter.
	// Default: 2m.
	PollRate time.Duration

	// VacuumTimeout - the maximum duration of a request, including retries.
	// Default: 1m.
	VacuumTimeout time.Duration
}

func (c *VacuumConfig) WithDefaults() *VacuumConfig {
	if c.PollRate == 0 {
		c.PollRate = 2 * time.Minute
	}
	if c.VacuumTimeout == 0 {
		c.VacuumTimeout = time.Minute
	}
	return c
}

func (c *VacuumConfig) Validate() error {
	if c.PollRate <= 0 {
		return fmt.Errorf("PollRate must be > 0, got %v", c.PollRate)
	}
	if c.VacuumTimeout <= 0 {
		return fmt.Errorf("VacuumTimeout must be > 0, got %v", c.VacuumTimeout)
	}
	return nil
}
