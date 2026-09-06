package producer

import (
	"fmt"
	"time"
)

type MetricsProducerConfig struct {
	// SessionFlushRate - pace of Run's flush tick: queued abandoned events
	// and changed session counters land once per tick.
	// Default: 30s.
	SessionFlushRate time.Duration
}

func (c *MetricsProducerConfig) WithDefaults() *MetricsProducerConfig {
	if c.SessionFlushRate == 0 {
		c.SessionFlushRate = 30 * time.Second
	}
	return c
}

func (c *MetricsProducerConfig) Validate() error {
	if c.SessionFlushRate <= 0 {
		return fmt.Errorf("SessionFlushRate must be > 0, got %v", c.SessionFlushRate)
	}
	return nil
}
