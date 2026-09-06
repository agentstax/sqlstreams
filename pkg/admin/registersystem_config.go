package admin

import (
	"fmt"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/agentstax/vulkan/pkg/system"
)

// RegisterSystemConfig is RegisterSystem's spec -- the system's own settings,
// the schedule each built-in alert is evaluated on, and the metrics
// collector's poll rate. Every field is optional.
type RegisterSystemConfig struct {
	// System - the control-plane settings scoped to no single topic.
	// Default: SystemConfig's own defaults.
	System *system.SystemConfig

	// PartitionCountAlert - the partition_count alert declaration.
	// Default: its own defaults.
	PartitionCountAlert *alert.PartitionCountAlertConfig

	// CompactionReadCostAlert - the compaction_read_cost alert declaration.
	// Default: its own defaults.
	CompactionReadCostAlert *alert.CompactionReadCostAlertConfig

	// WorkerLivenessAlert - the worker_liveness alert declaration.
	// Default: its own defaults.
	WorkerLivenessAlert *alert.WorkerLivenessAlertConfig

	// MetricsCollector - the metrics_collector worker declaration.
	// Default: its own defaults.
	MetricsCollector *metrics.MetricsCollectorWorkerConfig
}

func (c *RegisterSystemConfig) WithDefaults() *RegisterSystemConfig {
	if c.System == nil {
		c.System = &system.SystemConfig{}
	}
	c.System.WithDefaults()
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
	if c.MetricsCollector == nil {
		c.MetricsCollector = &metrics.MetricsCollectorWorkerConfig{}
	}
	c.MetricsCollector.WithDefaults()
	return c
}

// Validate runs after WithDefaults -- anything still out of range here was
// set by the caller, not left unset.
func (c *RegisterSystemConfig) Validate() error {
	if err := c.System.Validate(); err != nil {
		return fmt.Errorf("System: %w", err)
	}
	if err := c.PartitionCountAlert.Validate(); err != nil {
		return fmt.Errorf("PartitionCountAlert: %w", err)
	}
	if err := c.CompactionReadCostAlert.Validate(); err != nil {
		return fmt.Errorf("CompactionReadCostAlert: %w", err)
	}
	if err := c.WorkerLivenessAlert.Validate(); err != nil {
		return fmt.Errorf("WorkerLivenessAlert: %w", err)
	}
	if err := c.MetricsCollector.Validate(); err != nil {
		return fmt.Errorf("MetricsCollector: %w", err)
	}
	return nil
}
