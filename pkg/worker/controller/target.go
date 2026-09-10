package controller

import (
	"context"
	"fmt"

	"github.com/agentstax/sqlstreams/pkg/worker"
)

// A zero target requests shutdown at each instance's next heartbeat.
// Updating the target does not wait for those instances to stop.
func (c *WorkerController) UpdateTargetInstances(ctx context.Context, workerId int64, target worker.InstanceTarget) error {
	if workerId <= 0 {
		return fmt.Errorf("workerId must be > 0, got %d", workerId)
	}
	if err := target.Validate(); err != nil {
		return fmt.Errorf("target: %w", err)
	}
	return c.datastore.UpdateTargetInstances(ctx, workerId, int(target))
}
