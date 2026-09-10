package stream

import (
	"fmt"
	"time"
)

// StreamConfig is Register's spec -- separate from Stream so Register can grow
// (retention, etc.) without a signature change.
type StreamConfig struct {
	// PartitionSize - rows per partition.
	// Default: 1_000_000.
	//
	// Lower values give finer-grained retention drops at the cost of more
	// partitions to maintain. Tune down for low-volume streams, up for
	// high-throughput ones.
	// Ex: 10_000 for a low-volume audit stream, 5_000_000 for high-throughput ingest.
	PartitionSize int64

	// RetentionTTL - how long a message survives before the janitor may drop
	// or sweep it.
	// Default: 0 (keep every message indefinitely).
	//
	// Set this once a stream has a real expiry requirement.
	// Ex: 30 * 24 * time.Hour for a 30-day event stream.
	RetentionTTL time.Duration

	// AllowDropPastCommitted - if true, retention can drop data a lagging
	// consumer group hasn't committed yet (Kafka's default behavior).
	// Default: false.
	//
	// Set true only if a badly-lagging consumer should lose data rather than
	// block cleanup.
	// Ex: true for a metrics stream where staleness beats unbounded disk growth.
	AllowDropPastCommitted bool

	// IdempotencyKeyTTL - how long a produce-retry claim survives in
	// idempotency_key before the janitor sweeps it.
	// Default: 24h.
	//
	// Zero means the default, not "forever" -- WithDefaults resolves it
	// before the stream is ever registered. TTL only needs to cover your retry horizon,
	// not a retention window: 24h covers a webhook provider's retry day.
	// Every produce writes a claim row, minted key or not, so the TTL is
	// the claim table's size -- lower it for a stream whose producers never
	// retry across a restart.
	// Ex: 10 * time.Minute.
	IdempotencyKeyTTL time.Duration

	// EmptyCompactionHeadTTL - how long a compaction-head row with no current
	// head may stay idle before the stream janitor sweeps it.
	// Default: 1h.
	//
	// Zero means the default, not "forever" -- WithDefaults resolves it
	// before the stream is ever registered. Locking an empty head refreshes its activity;
	// the TTL never applies to a row that points at a head.
	EmptyCompactionHeadTTL time.Duration

	// DeliveryLogMode - which delivery outcomes write to delivery_log_<id>, the
	// per-attempt audit trail.
	// Default: DeliveryLogModeFailures (every outcome except success).
	//
	// DeliveryLogModeOff for a stream whose failure volume would make the extra
	// per-attempt write not worth paying for.
	// DeliveryLogModeAll when successes must be auditable per message
	// each success txn then also writes its 'success' row.
	DeliveryLogMode DeliveryLogMode

	// Janitor - retention cleanup settings.
	// Default: its own defaults.
	Janitor *JanitorConfig

	// Vacuum - key-table vacuum settings. New streams start with vacuum suspended.
	// Default: its own defaults.
	Vacuum *VacuumConfig
}

func (c *StreamConfig) WithDefaults() *StreamConfig {
	if c.PartitionSize == 0 {
		c.PartitionSize = 1_000_000
	}
	if c.IdempotencyKeyTTL == 0 {
		c.IdempotencyKeyTTL = 24 * time.Hour
	}
	if c.EmptyCompactionHeadTTL == 0 {
		c.EmptyCompactionHeadTTL = time.Hour
	}
	if c.DeliveryLogMode == "" {
		c.DeliveryLogMode = DeliveryLogModeFailures
	}
	if c.Janitor == nil {
		c.Janitor = &JanitorConfig{}
	}
	c.Janitor.WithDefaults()
	if c.Vacuum == nil {
		c.Vacuum = &VacuumConfig{}
	}
	c.Vacuum.WithDefaults()
	return c
}

func (c *StreamConfig) Validate() error {
	// 1 makes every id a partition boundary; <= 0 breaks the DDL range
	if c.PartitionSize < 2 {
		return fmt.Errorf("PartitionSize must be >= 2, got %d", c.PartitionSize)
	}
	if c.RetentionTTL < 0 {
		return fmt.Errorf("RetentionTTL must be >= 0, got %v", c.RetentionTTL)
	}
	if c.IdempotencyKeyTTL < 0 {
		return fmt.Errorf("IdempotencyKeyTTL must be >= 0, got %v", c.IdempotencyKeyTTL)
	}
	if c.EmptyCompactionHeadTTL <= 0 {
		return fmt.Errorf("EmptyCompactionHeadTTL must be > 0, got %v", c.EmptyCompactionHeadTTL)
	}
	if err := validateDeliveryLogMode(c.DeliveryLogMode); err != nil {
		return err
	}
	if err := c.Janitor.Validate(); err != nil {
		return fmt.Errorf("Janitor: %w", err)
	}
	if err := c.Vacuum.Validate(); err != nil {
		return fmt.Errorf("Vacuum: %w", err)
	}
	return nil
}

// ToStream builds the stream row a registration writes from the resolved config.
func (c *StreamConfig) ToStream(id int64, systemId int64, name string) *Stream {
	return &Stream{
		Id:                     id,
		SystemId:               systemId,
		Name:                   name,
		PartitionSize:          c.PartitionSize,
		RetentionTTL:           c.RetentionTTL,
		AllowDropPastCommitted: c.AllowDropPastCommitted,
		IdempotencyKeyTTL:      c.IdempotencyKeyTTL,
		EmptyCompactionHeadTTL: c.EmptyCompactionHeadTTL,
		DeliveryLogMode:        c.DeliveryLogMode,
	}
}

// ***************
// *** HELPERS ***
// ***************

func validateDeliveryLogMode(deliveryLogMode DeliveryLogMode) error {
	switch deliveryLogMode {
	case DeliveryLogModeOff, DeliveryLogModeFailures, DeliveryLogModeAll:
		return nil
	}
	return fmt.Errorf("DeliveryLogMode must be %q, %q, or %q, got %q", DeliveryLogModeOff, DeliveryLogModeFailures, DeliveryLogModeAll, deliveryLogMode)
}
