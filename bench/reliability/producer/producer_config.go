package producer

import "fmt"

// ProducerConfig selects the produce path and encoded payload size.
type ProducerConfig struct {
	AutomaticBatching bool
	PayloadBytes      int // 0 keeps the identity-only payload.
}

func (c *ProducerConfig) WithDefaults() *ProducerConfig { return c }

func (c *ProducerConfig) Validate() error {
	if c.PayloadBytes != 0 && c.PayloadBytes < 128 {
		return fmt.Errorf("PayloadBytes must be 0 or >= 128, got %d", c.PayloadBytes)
	}
	return nil
}
