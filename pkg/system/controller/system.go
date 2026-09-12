package controller

import (
	"context"

	"github.com/allegedlyreliable/sqlstreams/pkg/system"
)

// Register creates the shared control-plane tables and resolves the
// singleton system row. Idempotent.
func (c *SystemController) Register(ctx context.Context) (*system.System, error) {
	registered, err := c.datastore.Register(ctx)
	if err != nil {
		return nil, err
	}

	return toSystem(registered), nil
}

// Get returns the singleton system config, or (nil, nil) if the system
// hasn't been registered.
func (c *SystemController) Get(ctx context.Context) (*system.System, error) {
	found, err := c.datastore.Get(ctx)
	if err != nil || found == nil {
		return nil, err
	}
	return toSystem(found), nil
}

// Delete drops the shared control-plane tables -- every table
// Register creates. Callers drop the per-stream tables first; a stream
// still registered when this runs leaves its physical tables orphaned.
func (c *SystemController) Delete(ctx context.Context) error {
	return c.datastore.Delete(ctx)
}
