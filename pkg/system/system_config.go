package system

import (
	"fmt"

	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/metric"
)

// SystemConfig declares the built-in alert settings and metrics collector's
// poll rate. Every field is optional.
type SystemConfig struct {
	// PartitionCountAlert - the partition_count alert declaration.
	// Default: its own defaults.
	PartitionCountAlert *alert.PartitionCountAlertConfig

	// CompactionReadCostAlert - the compaction_read_cost alert declaration.
	// Default: its own defaults.
	CompactionReadCostAlert *alert.CompactionReadCostAlertConfig

	// WorkerLivenessAlert - the worker_liveness alert declaration.
	// Default: its own defaults.
	WorkerLivenessAlert *alert.WorkerLivenessAlertConfig

	// MetricCollectorProgressAlert declares the collector-progress check. Default: its own defaults.
	MetricCollectorProgressAlert *alert.MetricCollectorProgressAlertConfig

	// MetricCollector - the metrics_collector worker declaration.
	// Default: its own defaults.
	MetricCollector *metric.MetricCollectorWorkerConfig
}

func (c *SystemConfig) WithDefaults() *SystemConfig {
	if c.PartitionCountAlert == nil {
		c.PartitionCountAlert = &alert.PartitionCountAlertConfig{}
	}
	c.PartitionCountAlert.WithDefaults()
	if c.CompactionReadCostAlert == nil {
		c.CompactionReadCostAlert = &alert.CompactionReadCostAlertConfig{}
	}
	c.CompactionReadCostAlert.WithDefaults()
	if c.WorkerLivenessAlert == nil {
		c.WorkerLivenessAlert = &alert.WorkerLivenessAlertConfig{}
	}
	c.WorkerLivenessAlert.WithDefaults()
	if c.MetricCollectorProgressAlert == nil {
		c.MetricCollectorProgressAlert = &alert.MetricCollectorProgressAlertConfig{}
	}
	c.MetricCollectorProgressAlert.WithDefaults()
	if c.MetricCollector == nil {
		c.MetricCollector = &metric.MetricCollectorWorkerConfig{}
	}
	c.MetricCollector.WithDefaults()
	return c
}

func (c *SystemConfig) Validate() error {
	if err := c.PartitionCountAlert.Validate(); err != nil {
		return fmt.Errorf("PartitionCountAlert: %w", err)
	}
	if err := c.CompactionReadCostAlert.Validate(); err != nil {
		return fmt.Errorf("CompactionReadCostAlert: %w", err)
	}
	if err := c.WorkerLivenessAlert.Validate(); err != nil {
		return fmt.Errorf("WorkerLivenessAlert: %w", err)
	}
	if err := c.MetricCollectorProgressAlert.Validate(); err != nil {
		return fmt.Errorf("MetricCollectorProgressAlert: %w", err)
	}
	if err := c.MetricCollector.Validate(); err != nil {
		return fmt.Errorf("MetricCollector: %w", err)
	}
	return nil
}
