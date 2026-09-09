package producer

import (
	"fmt"
	"time"
)

type MetricProducerConfig struct {
	// SessionFlushRate - pace of Run's flush tick: queued abandoned events
	// and changed session counters land once per tick.
	// Default: 30s.
	SessionFlushRate time.Duration
}

func (c *MetricProducerConfig) WithDefaults() *MetricProducerConfig {
	if c.SessionFlushRate == 0 {
		c.SessionFlushRate = 30 * time.Second
	}
	return c
}

func (c *MetricProducerConfig) Validate() error {
	if c.SessionFlushRate <= 0 {
		return fmt.Errorf("SessionFlushRate must be > 0, got %v", c.SessionFlushRate)
	}
	return nil
}
