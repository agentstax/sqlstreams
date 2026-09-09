package controller

import (
	"context"
	"fmt"
	"time"
)

// SweepExpiredIdempotencyKeys deletes idempotency claims older than ttl in
// batches.
func (c *JanitorController) SweepExpiredIdempotencyKeys(ctx context.Context, streamId int64, ttl time.Duration, batchSize int) error {
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if batchSize <= 0 {
		return fmt.Errorf("batchSize must be > 0, got %d", batchSize)
	}

	return c.datastore.SweepExpiredIdempotencyKeys(ctx, streamId, ttl, batchSize)
}
