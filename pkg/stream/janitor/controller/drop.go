package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
)

// DropExpiredPartitions drops each surviving partition whose newest row is
// past ttl, skipping the active partition and (unless overridden) anything a
// lagging group hasn't committed past yet. ttl <= 0 means retention is
// disabled -- the call is a no-op.
func (c *JanitorController) DropExpiredPartitions(ctx context.Context, streamId int64, partitionSize int64, ttl time.Duration, allowDropPastCommitted bool, deliveryLogMode stream.DeliveryLogMode) error {
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if partitionSize <= 0 {
		return fmt.Errorf("partitionSize must be > 0, got %d", partitionSize)
	}

	return c.datastore.DropExpiredPartitions(ctx, streamId, partitionSize, ttl, allowDropPastCommitted, deliveryLogMode)
}
