package controller

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
)

// warnDivisor halves the lock ceiling so the alert leaves headroom to act
// before Destroy starts failing.
const warnDivisor = 2

// Evaluate reads the collector's retained partition count. Missing evidence
// returns an error; threshold 0 uses half the live lock ceiling.
func (c *PartitionCountController) Evaluate(ctx context.Context, owner *common.Owner, threshold int64) (*alert.Alert, error) {
	if owner == nil {
		return nil, errors.New("owner must not be nil")
	}
	if threshold < 0 {
		return nil, fmt.Errorf("threshold must be >= 0, got %d", threshold)
	}

	ceiling, err := c.datastore.PartitionLockCeiling(ctx)
	if err != nil {
		return nil, err
	}
	if threshold == 0 {
		threshold = ceiling / warnDivisor
	}

	key := metrics.MeasurementKey(metrics.MetricTopicPartitions.Name, map[string]string{"topic": owner.Name})
	stored, err := c.metrics.GetMeasurement(ctx, key)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, fmt.Errorf("partition count measurement is missing for topic %q", owner.Name)
	}
	value := stored.Message.Value
	if math.IsNaN(value) || value < 0 || value >= math.MaxInt64 || math.Trunc(value) != value {
		return nil, fmt.Errorf("partition count must be a non-negative integer below the int64 limit, got %g", value)
	}
	count := int64(value)
	if count < threshold {
		return nil, nil
	}
	return newPartitionCountAlert(owner, count, ceiling, threshold, stored.CreatedAt)
}
