package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
)

// Evaluate returns healthy when every owned worker is claimed or suspended.
// Threshold is unused; expired instance rows do not retain unclaimed duration.
func (c *WorkerLivenessController) Evaluate(ctx context.Context, owner *common.Owner, threshold int64) (*alert.AlertEvaluationResult, error) {
	if owner == nil {
		return nil, errors.New("owner must not be nil")
	}
	if threshold < 0 {
		return nil, fmt.Errorf("threshold must be >= 0, got %d", threshold)
	}

	snapshots, err := c.metrics.WorkerSnapshots(ctx)
	if err != nil {
		return nil, err
	}

	// a group's rows carry the group as owner and resolve to its topic
	var unclaimed []metrics.WorkerSnapshot
	for _, snapshot := range snapshots {
		if snapshot.Owner.TopicId == owner.TopicId && snapshot.Status == metrics.WorkerUnclaimed {
			unclaimed = append(unclaimed, snapshot)
		}
	}
	if len(unclaimed) == 0 {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateHealthy, nil)
	}
	finding, err := newWorkerLivenessAlert(owner, unclaimed, time.Now())
	if err != nil {
		return nil, err
	}
	return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateActive, finding)
}
