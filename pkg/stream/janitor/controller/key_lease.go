package controller

import (
	"context"
	"fmt"
)

// SweepExpiredKeyLeases deletes expired message_key_lease rows in batches.
func (c *JanitorController) SweepExpiredKeyLeases(ctx context.Context, streamId int64, batchSize int) error {
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if batchSize <= 0 {
		return fmt.Errorf("batchSize must be > 0, got %d", batchSize)
	}

	return c.datastore.SweepExpiredKeyLeases(ctx, streamId, batchSize)
}
