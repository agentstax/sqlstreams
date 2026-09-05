package controller

import (
	"context"
	"fmt"
	"time"
)

// SweepExpiredEmptyCompactionHeads deletes idle compaction-head rows that do
// not point at a message, in batches.
func (c *JanitorController) SweepExpiredEmptyCompactionHeads(ctx context.Context, topicId int64, ttl time.Duration, batchSize int) error {
	if topicId <= 0 {
		return fmt.Errorf("topicId must be > 0, got %d", topicId)
	}
	if ttl <= 0 {
		return fmt.Errorf("ttl must be > 0, got %v", ttl)
	}
	if batchSize <= 0 {
		return fmt.Errorf("batchSize must be > 0, got %d", batchSize)
	}

	return c.datastore.SweepExpiredEmptyCompactionHeads(ctx, topicId, ttl, batchSize)
}
