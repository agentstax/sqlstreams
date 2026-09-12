package controller

import (
	"context"
	"fmt"

	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
)

func (c *StreamController) AssertSchemaSupported(ctx context.Context, systemId int64, streamId int64) error {
	if systemId <= 0 {
		return fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}

	return c.migrateController.AssertStreamSchemaSupported(ctx, systemId, streamId)
}

// AssertSchemaSupportedInTx gates a stream through tx.
func (c *StreamController) AssertSchemaSupportedInTx(ctx context.Context, tx iDatastore.Tx, systemId int64, streamId int64) error {
	return c.migrateController.AssertStreamSchemaSupportedInTx(ctx, tx, systemId, streamId)
}
