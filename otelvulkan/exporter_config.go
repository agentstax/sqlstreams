package otelvulkan

import (
	"fmt"
	"time"
)

type ExporterConfig struct {
	// CollectTimeout bounds the Postgres reads a scrape drives -- the
	// instrument registration pass and the observation callback. A scrape
	// request may carry no deadline of its own.
	// Default: 5s.
	CollectTimeout time.Duration
}

func (c *ExporterConfig) WithDefaults() *ExporterConfig {
	if c.CollectTimeout == 0 {
		c.CollectTimeout = 5 * time.Second
	}
	return c
}

// Validate runs after WithDefaults -- anything still out of range here was
// set by the caller, not left unset.
func (c *ExporterConfig) Validate() error {
	if c.CollectTimeout <= 0 {
		return fmt.Errorf("CollectTimeout must be > 0, got %v", c.CollectTimeout)
	}
	return nil
}
