package producer

import "fmt"

// ProducerConfig selects the produce path and encoded payload size.
type ProducerConfig struct {
	// DisableMessageRecording replaces per-message records with counters. Default: false.
	DisableMessageRecording bool
	// AutomaticBatching uses the library's automatic batcher. Default: false.
	AutomaticBatching bool
	// PayloadBytes sets the encoded message size. Default: 0 (identity-only payload).
	PayloadBytes int
}

func (c *ProducerConfig) WithDefaults() *ProducerConfig { return c }

func (c *ProducerConfig) Validate() error {
	if c.PayloadBytes != 0 && c.PayloadBytes < 128 {
		return fmt.Errorf("PayloadBytes must be 0 or >= 128, got %d", c.PayloadBytes)
	}
	return nil
}
