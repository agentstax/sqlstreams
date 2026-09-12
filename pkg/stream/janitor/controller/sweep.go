package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// SweepExpiredPartitions drains the ttl-expired prefix of every surviving
// partition -- covers the low-volume tail that never fills a partition wide
// enough to earn a whole-partition drop. Each batch requires its oldest row
// to exceed ttl plus gracePeriod, then deletes rows using the ttl cutoff.
// ttl <= 0 disables retention, making the call a no-op.
func (c *JanitorController) SweepExpiredPartitions(ctx context.Context, streamId int64, partitionSize int64, ttl time.Duration, gracePeriod time.Duration, allowDropPastCommitted bool, batchSize int, deliveryLogMode stream.DeliveryLogMode) error {
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if partitionSize <= 0 {
		return fmt.Errorf("partitionSize must be > 0, got %d", partitionSize)
	}
	if gracePeriod < 0 {
		return fmt.Errorf("gracePeriod must be >= 0, got %v", gracePeriod)
	}
	if batchSize <= 0 {
		return fmt.Errorf("batchSize must be > 0, got %d", batchSize)
	}

	return c.datastore.SweepExpiredPartitions(ctx, streamId, partitionSize, ttl, gracePeriod, allowDropPastCommitted, batchSize, deliveryLogMode)
}
