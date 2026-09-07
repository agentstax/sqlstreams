package controller

import (
	"context"
	"fmt"
	"time"
)

// SweepExpiredInstanceLogs retains each snapshot until ttl after its recorded lease expiry.
func (c *WorkerController) SweepExpiredInstanceLogs(ctx context.Context, ttl time.Duration) (int64, error) {
	if ttl <= 0 {
		return 0, fmt.Errorf("ttl must be > 0, got %v", ttl)
	}
	return c.datastore.SweepExpiredInstanceLogs(ctx, ttl)
}
