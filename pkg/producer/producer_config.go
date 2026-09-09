package producer

import (
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/produce/batcher"
)

// ProducerConfig is one producer instance's process-local settings: its
// message defaults and batching. Nothing durable -- a producer has no row.
type ProducerConfig struct {
	// Message - this producer's default MessageOptions, merged UNDER every
	// produce: a field the per-produce ProduceOptions.Message leaves unset
	// takes its value from here before the message is stored. Fields unset in
	// both stay unset -- the consumer decides.
	// Default: nil (no producer-side defaults).
	Message *common.MessageOptions

	// Batch - knobs for the shared-transaction batching of concurrent Produce
	// calls. See batcher.BatcherConfig for fields and defaults.
	Batch batcher.BatcherConfig

	// SlowProduceThreshold - a produce call running longer than this logs a
	// warn line with its duration. ProduceFunc and ProduceFuncInTx include
	// the caller's closure; both InTx verbs end at the insert, before the
	// caller's commit.
	// Default: 0 (disabled).
	SlowProduceThreshold time.Duration
}

func (c *ProducerConfig) WithDefaults() *ProducerConfig {
	c.Batch.WithDefaults()
	return c
}

func (c *ProducerConfig) Validate() error {
	if c.SlowProduceThreshold < 0 {
		return fmt.Errorf("SlowProduceThreshold must be >= 0, got %v", c.SlowProduceThreshold)
	}
	if err := c.Message.Validate(); err != nil {
		return fmt.Errorf("Message: %w", err)
	}
	if err := c.Batch.Validate(); err != nil {
		return fmt.Errorf("Batch: %w", err)
	}
	return nil
}
