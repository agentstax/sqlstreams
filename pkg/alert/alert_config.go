package alert

import "fmt"

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
}

func (c *PartitionCountAlertConfig) WithDefaults() *PartitionCountAlertConfig {
	if c.ScheduleExpression == "" {
		c.ScheduleExpression = "@hourly"
	}
	return c
}

// Validate runs after WithDefaults; the expression is parsed where the
// schedule is declared.
func (c *PartitionCountAlertConfig) Validate() error {
	if c.Threshold < 0 {
		return fmt.Errorf("Threshold must be >= 0, got %d", c.Threshold)
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
}

func (c *CompactionReadCostAlertConfig) WithDefaults() *CompactionReadCostAlertConfig {
	if c.ScheduleExpression == "" {
		c.ScheduleExpression = "@hourly"
	}
	return c
}

// Validate runs after WithDefaults; the expression is parsed where the
// schedule is declared.
func (c *CompactionReadCostAlertConfig) Validate() error {
	if c.Threshold < 0 {
		return fmt.Errorf("Threshold must be >= 0, got %d", c.Threshold)
	}
	return nil
}

// WorkerLivenessAlertConfig declares how the worker_liveness alert is
// evaluated.
type WorkerLivenessAlertConfig struct {
	// ScheduleExpression - how often the alert is evaluated, a cron expression.
	// Default: @hourly.
	ScheduleExpression string
}

func (c *WorkerLivenessAlertConfig) WithDefaults() *WorkerLivenessAlertConfig {
	if c.ScheduleExpression == "" {
		c.ScheduleExpression = "@hourly"
	}
	return c
}

// Validate runs after WithDefaults; the expression is parsed where the
// schedule is declared.
func (c *WorkerLivenessAlertConfig) Validate() error {
	return nil
}
