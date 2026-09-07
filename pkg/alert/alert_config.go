package alert

import (
	"fmt"
	"math"
	"time"
)

// PartitionCountAlertConfig declares how the partition_count alert is
// evaluated and the count it alerts at.
type PartitionCountAlertConfig struct {
	// ScheduleExpression - how often the alert is evaluated, a cron expression.
	// Default: @hourly.
	ScheduleExpression string

	// Threshold - the partition count on one topic at or above which the
	// alert is published.
	// Default: 0, which measures against half the lock ceiling Postgres
	// reports.
	Threshold int64

	// PendingDuration is the required consecutive unhealthy sample span. Default: 2m.
	PendingDuration time.Duration

	// MaximumGap limits sample gaps and the newest sample's age. Default: 2m.
	MaximumGap time.Duration

	// DisablePending permits immediate activation from fresh unhealthy evidence. Default: false.
	DisablePending bool
}

func (c *PartitionCountAlertConfig) WithDefaults() *PartitionCountAlertConfig {
	if c.ScheduleExpression == "" {
		c.ScheduleExpression = "@hourly"
	}
	if c.PendingDuration == 0 {
		c.PendingDuration = 2 * time.Minute
	}
	if c.MaximumGap == 0 {
		c.MaximumGap = 2 * time.Minute
	}
	return c
}

// The schedule expression is parsed where the schedule is declared, not here.
func (c *PartitionCountAlertConfig) Validate() error {
	if c.Threshold < 0 {
		return fmt.Errorf("Threshold must be >= 0, got %d", c.Threshold)
	}
	if c.PendingDuration <= 0 {
		return fmt.Errorf("PendingDuration must be > 0, got %v", c.PendingDuration)
	}
	if c.MaximumGap <= 0 {
		return fmt.Errorf("MaximumGap must be > 0, got %v", c.MaximumGap)
	}
	if !c.DisablePending && c.MaximumGap > (math.MaxInt64-c.PendingDuration)/2 {
		return fmt.Errorf("MaximumGap must be <= %v for PendingDuration %v, got %v", (time.Duration(math.MaxInt64)-c.PendingDuration)/2, c.PendingDuration, c.MaximumGap)
	}
	return nil
}

// CompactionReadCostAlertConfig declares how the compaction_read_cost alert is
// evaluated and the cost it alerts at.
type CompactionReadCostAlertConfig struct {
	// ScheduleExpression - how often the alert is evaluated, a cron expression.
	// Default: @hourly.
	ScheduleExpression string

	// Threshold - the read cost at or above which the alert is published.
	// Default: 0, the check's own ceiling.
	Threshold int64

	// PendingDuration is the required consecutive unhealthy sample span. Default: 2m.
	PendingDuration time.Duration

	// MaximumGap limits sample gaps and the newest sample's age. Default: 2m.
	MaximumGap time.Duration

	// DisablePending permits immediate activation from fresh unhealthy evidence. Default: false.
	DisablePending bool
}

func (c *CompactionReadCostAlertConfig) WithDefaults() *CompactionReadCostAlertConfig {
	if c.ScheduleExpression == "" {
		c.ScheduleExpression = "@hourly"
	}
	if c.PendingDuration == 0 {
		c.PendingDuration = 2 * time.Minute
	}
	if c.MaximumGap == 0 {
		c.MaximumGap = 2 * time.Minute
	}
	return c
}

// The schedule expression is parsed where the schedule is declared, not here.
func (c *CompactionReadCostAlertConfig) Validate() error {
	if c.Threshold < 0 {
		return fmt.Errorf("Threshold must be >= 0, got %d", c.Threshold)
	}
	if c.PendingDuration <= 0 {
		return fmt.Errorf("PendingDuration must be > 0, got %v", c.PendingDuration)
	}
	if c.MaximumGap <= 0 {
		return fmt.Errorf("MaximumGap must be > 0, got %v", c.MaximumGap)
	}
	if !c.DisablePending && c.MaximumGap > (math.MaxInt64-c.PendingDuration)/2 {
		return fmt.Errorf("MaximumGap must be <= %v for PendingDuration %v, got %v", (time.Duration(math.MaxInt64)-c.PendingDuration)/2, c.PendingDuration, c.MaximumGap)
	}
	return nil
}

// WorkerLivenessAlertConfig declares how the worker_liveness alert is
// evaluated.
type WorkerLivenessAlertConfig struct {
	// ScheduleExpression - how often the alert is evaluated, a cron expression.
	// Default: @hourly.
	ScheduleExpression string

	// PendingDuration is the required consecutive unhealthy sample span. Default: 2m.
	PendingDuration time.Duration

	// MaximumGap limits sample gaps and the newest sample's age. Default: 2m.
	MaximumGap time.Duration

	// DisablePending permits immediate activation from fresh unhealthy evidence. Default: false.
	DisablePending bool
}

func (c *WorkerLivenessAlertConfig) WithDefaults() *WorkerLivenessAlertConfig {
	if c.ScheduleExpression == "" {
		c.ScheduleExpression = "@hourly"
	}
	if c.PendingDuration == 0 {
		c.PendingDuration = 2 * time.Minute
	}
	if c.MaximumGap == 0 {
		c.MaximumGap = 2 * time.Minute
	}
	return c
}

// The schedule expression is parsed where the schedule is declared, not here.
func (c *WorkerLivenessAlertConfig) Validate() error {
	if c.PendingDuration <= 0 {
		return fmt.Errorf("PendingDuration must be > 0, got %v", c.PendingDuration)
	}
	if c.MaximumGap <= 0 {
		return fmt.Errorf("MaximumGap must be > 0, got %v", c.MaximumGap)
	}
	if !c.DisablePending && c.MaximumGap > (math.MaxInt64-c.PendingDuration)/2 {
		return fmt.Errorf("MaximumGap must be <= %v for PendingDuration %v, got %v", (time.Duration(math.MaxInt64)-c.PendingDuration)/2, c.PendingDuration, c.MaximumGap)
	}
	return nil
}
