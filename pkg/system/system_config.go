package system

import (
	"fmt"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/metrics"
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

	// MetricsCollectorProgressAlert declares the collector-progress check. Default: its own defaults.
	MetricsCollectorProgressAlert *alert.MetricsCollectorProgressAlertConfig

	// MetricsCollector - the metrics_collector worker declaration.
	// Default: its own defaults.
	MetricsCollector *metrics.MetricsCollectorWorkerConfig
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
	if c.MetricsCollectorProgressAlert == nil {
		c.MetricsCollectorProgressAlert = &alert.MetricsCollectorProgressAlertConfig{}
	}
	c.MetricsCollectorProgressAlert.WithDefaults()
	if c.MetricsCollector == nil {
		c.MetricsCollector = &metrics.MetricsCollectorWorkerConfig{}
	}
	c.MetricsCollector.WithDefaults()
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
	if err := c.MetricsCollectorProgressAlert.Validate(); err != nil {
		return fmt.Errorf("MetricsCollectorProgressAlert: %w", err)
	}
	if err := c.MetricsCollector.Validate(); err != nil {
		return fmt.Errorf("MetricsCollector: %w", err)
	}
	return nil
}
