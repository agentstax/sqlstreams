package controller

import (
	"context"
	"fmt"
	"time"
)

// SweepExpiredEmptyCompactionHeads deletes idle compaction-head rows that do
// not point at a message, in batches.
func (c *JanitorController) SweepExpiredEmptyCompactionHeads(ctx context.Context, streamId int64, ttl time.Duration, batchSize int) error {
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if ttl <= 0 {
		return fmt.Errorf("ttl must be > 0, got %v", ttl)
	}
	if batchSize <= 0 {
		return fmt.Errorf("batchSize must be > 0, got %d", batchSize)
	}

	return c.datastore.SweepExpiredEmptyCompactionHeads(ctx, streamId, ttl, batchSize)
}
