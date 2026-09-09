package scheduler

import (
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
)

// SchedulerConfig is a schedule's declared delivery semantics, stored on
// its row: how each message it produces runs.
type SchedulerConfig struct {
	// Timeout - how long one message's delivery may run.
	// Default: 30s.
	Timeout time.Duration

	// Concurrency - whether a message runs while a previous one is still
	// running (parallel) or waits for it (exclusive).
	// Default: parallel.
	Concurrency common.ConcurrencyPolicy

	// Metadata - marshaled to opaque JSON stored on the row and shown by
	// `sqlstreams schedule get`; it is not part of the produced message.
	// Default: nil, stored as {}.
	Metadata any
}

func (c *SchedulerConfig) WithDefaults() *SchedulerConfig {
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.Concurrency == "" {
		c.Concurrency = common.ConcurrencyParallel
	}
	return c
}

func (c *SchedulerConfig) Validate() error {
	if c.Timeout <= 0 {
		return fmt.Errorf("Timeout must be > 0, got %v", c.Timeout)
	}
	if err := c.Concurrency.Validate(); err != nil {
		return fmt.Errorf("Concurrency: %w", err)
	}
	return nil
}
