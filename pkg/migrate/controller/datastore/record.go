package datastore

import (
	"context"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
)

func (d *MigrateDatastore) recordSuccess(ctx context.Context, q datastore.Querier, owner *common.Owner, version int64, minCompatibleVersion int64) error {
	columns := datastore.NewOwnerColumns(*owner)

	sql := fmt.Sprintf(`
		-- sqlstreams: migrate.recordSuccess
		INSERT INTO %[1]s.migration_log (system_id, stream_id, consumer_group_id, version, min_compatible_version, status) VALUES ($1, $2, $3, $4, $5, 'success');
	`, d.Datastore.Schema)
	_, err := q.Exec(ctx, sql,
		columns.SystemId, columns.StreamId, columns.ConsumerGroupId, version, minCompatibleVersion)
	return err
}

// TryRecordFailure commits a diagnostic failure row after a step rolled back --
// best-effort, on a fresh context so the cancel that caused the failure doesn't
// also drop the record.
func (d *MigrateDatastore) TryRecordFailure(ctx context.Context, q datastore.Querier, owner *common.Owner, version int64, cause error) {
	columns := datastore.NewOwnerColumns(*owner)

	ctx = context.WithoutCancel(ctx)
	err := d.DatastoreRetry.WrapNonIdempotent(ctx, func() error {
		sql := fmt.Sprintf(`
			-- sqlstreams: migrate.TryRecordFailure
			INSERT INTO %[1]s.migration_log (system_id, stream_id, consumer_group_id, version, status, error) VALUES ($1, $2, $3, $4, 'failure', $5);
		`, d.Datastore.Schema)
		_, e := q.Exec(ctx, sql,
			columns.SystemId, columns.StreamId, columns.ConsumerGroupId, version, cause.Error())
		return e
	})
	if err != nil {
		d.Logger.ErrorContext(ctx, "could not record migration failure", "owner", owner.Name, "owner_kind", owner.Kind(), "stream_id", owner.StreamId, "group_id", owner.ConsumerGroupId, "version", version, "cause", cause, "error", err)
	}
}
