package alert

import (
	"fmt"
	"math"
	"time"
)

// MetricsCollectorProgressAlertConfig declares the installation's collector-progress check.
type MetricsCollectorProgressAlertConfig struct {
	// ScheduleExpression controls how often collector progress is checked. Default: @every 1m.
	ScheduleExpression string

	// MaximumAge is how old a completion may be. Default: 0, max(2m, three declared collector poll intervals).
	MaximumAge time.Duration

	// PendingDuration is the required unhealthy duration during continuous manager lease coverage. Default: 2m.
	PendingDuration time.Duration

	// DisablePending permits immediate activation with current manager lease coverage. Default: false.
	DisablePending bool
}

func (c *MetricsCollectorProgressAlertConfig) WithDefaults() *MetricsCollectorProgressAlertConfig {
	if c.ScheduleExpression == "" {
		c.ScheduleExpression = "@every 1m"
	}
	if c.PendingDuration == 0 {
		c.PendingDuration = 2 * time.Minute
	}
	return c
}

func (c *MetricsCollectorProgressAlertConfig) Validate() error {
	if c.MaximumAge < 0 {
		return fmt.Errorf("MaximumAge must be >= 0, got %v", c.MaximumAge)
	}
	if c.PendingDuration <= 0 {
		return fmt.Errorf("PendingDuration must be > 0, got %v", c.PendingDuration)
	}
	return nil
}

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
