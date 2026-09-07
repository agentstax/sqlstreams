package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
)

// warnPartitions is where one never-superseded key's replay, at ~10µs per
// partition, crosses ~100ms.
const warnPartitions = 10_000

// Evaluate returns healthy when compaction is absent or below the threshold.
// Threshold 0 uses the default warnPartitions.
func (c *CompactionReadCostController) Evaluate(ctx context.Context, owner *common.Owner, threshold int64) (*alert.AlertEvaluationResult, error) {
	if owner == nil {
		return nil, errors.New("owner must not be nil")
	}
	if threshold < 0 {
		return nil, fmt.Errorf("threshold must be >= 0, got %d", threshold)
	}
	if threshold == 0 {
		threshold = warnPartitions
	}

	compacted, err := c.datastore.IsCompacted(ctx, owner.TopicId)
	if err != nil {
		return nil, err
	}

	// only compacted topics carry a read cost
	if !compacted {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateHealthy, nil)
	}

	count, err := c.datastore.PartitionCount(ctx, owner.TopicId)
	if err != nil {
		return nil, err
	}
	if count < threshold {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateHealthy, nil)
	}
	finding, err := newCompactionReadCostAlert(owner, count, threshold, time.Now())
	if err != nil {
		return nil, err
	}
	return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateActive, finding)
}
