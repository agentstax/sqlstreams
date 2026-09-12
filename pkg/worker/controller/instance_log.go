package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
)

// GetInstanceHistory includes intervals crossing the window start, as of database time.
func (c *WorkerController) GetInstanceHistory(ctx context.Context, workerId int64, window time.Duration) (*worker.WorkerInstanceHistory, error) {
	if workerId <= 0 {
		return nil, fmt.Errorf("workerId must be > 0, got %d", workerId)
	}
	if window <= 0 {
		return nil, fmt.Errorf("window must be > 0, got %v", window)
	}

	current, err := c.datastore.CurrentTime(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := c.datastore.ListInstanceSnapshots(ctx, workerId, current.Add(-window), current)
	if err != nil {
		return nil, err
	}
	return toWorkerInstanceHistory(current, rows), nil
}

// SweepExpiredInstanceLogs retains each snapshot until ttl after its recorded lease expiry.
func (c *WorkerController) SweepExpiredInstanceLogs(ctx context.Context, ttl time.Duration) (int64, error) {
	if ttl <= 0 {
		return 0, fmt.Errorf("ttl must be > 0, got %v", ttl)
	}
	return c.datastore.SweepExpiredInstanceLogs(ctx, ttl)
}
