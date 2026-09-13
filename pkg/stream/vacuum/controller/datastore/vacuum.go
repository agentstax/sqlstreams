package datastore

import (
	"context"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// VacuumIdempotencyKeys makes deleted key storage reusable and refreshes planner statistics.
func (d *VacuumDatastore) VacuumIdempotencyKeys(ctx context.Context, streamId int64) error {
	return d.DatastoreRetry.WrapIdempotent(ctx, func() error {
		return d.vacuumIdempotencyKeys(ctx, streamId)
	})
}

func (d *VacuumDatastore) vacuumIdempotencyKeys(ctx context.Context, streamId int64) error {
	// VACUUM must execute outside a transaction block.
	sql := fmt.Sprintf(`
		-- sqlstreams: streamvacuum.vacuumIdempotencyKeys
		VACUUM (ANALYZE) %[1]s.%[2]s
	`, d.Datastore.Schema, stream.IdempotencyKeyTable(streamId))
	_, err := d.Datastore.Pool.Exec(ctx, sql)
	return err
}
