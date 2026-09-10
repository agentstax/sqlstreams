package controller

import (
	"context"
	"fmt"
)

func (c *VacuumController) VacuumIdempotencyKeys(ctx context.Context, streamId int64) error {
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}

	return c.datastore.VacuumIdempotencyKeys(ctx, streamId)
}
