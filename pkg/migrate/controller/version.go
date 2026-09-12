package controller

import (
	"context"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
)

// SystemVersion reads the system's current schema version from migration_log.
// Returns ErrNotRegistered if there is no baseline record.
func (c *Controller) SystemVersion(ctx context.Context, systemId int64) (int64, error) {
	if systemId <= 0 {
		return 0, fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	state, err := c.datastore.SystemSchemaState(ctx, systemId)
	if err != nil {
		return 0, err
	}
	return state.Version, nil
}

// StreamVersion reads a stream's current schema version from migration_log.
// Returns ErrNotRegistered if there is no baseline record.
func (c *Controller) StreamVersion(ctx context.Context, streamId int64) (int64, error) {
	if streamId <= 0 {
		return 0, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	state, err := c.datastore.StreamSchemaState(ctx, streamId)
	if err != nil {
		return 0, err
	}
	return state.Version, nil
}

// SystemOwner resolves the singleton system row to its owner.
// Returns ErrNotRegistered if RegisterSystem hasn't run.
func (c *Controller) SystemOwner(ctx context.Context) (*common.Owner, error) {
	return c.datastore.SystemOwner(ctx)
}
