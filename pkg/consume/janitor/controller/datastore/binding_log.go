package datastore

import (
	"context"
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// SweepExpiredWaitingDeclarations deletes waiting binding_config_log rows whose
// attempt ran more than ttl ago -- one batched DELETE per stream's table, at
// most batchSize rows each -- and returns how many were deleted in total.
func (d *JanitorDatastore) SweepExpiredWaitingDeclarations(ctx context.Context, ttl time.Duration, batchSize int) (int64, error) {
	var swept int64
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		swept, err = d.sweepExpiredWaitingDeclarations(ctx, ttl, batchSize)
		return err
	})
	return swept, err
}

func (d *JanitorDatastore) sweepExpiredWaitingDeclarations(ctx context.Context, ttl time.Duration, batchSize int) (int64, error) {
	streamIds, err := d.listGroupStreamIds(ctx)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-ttl)

	var swept int64
	for _, streamId := range streamIds {
		streamSwept, err := d.sweepStreamWaitingDeclarations(ctx, streamId, cutoff, batchSize)
		if err != nil {
			return 0, err
		}
		swept += streamSwept
	}
	return swept, nil
}

func (d *JanitorDatastore) sweepStreamWaitingDeclarations(ctx context.Context, streamId int64, cutoff time.Time, batchSize int) (int64, error) {
	// a declarer's newest waiting id is protected even past the cutoff, so
	// a dead waiter stays visible in listings. Installed rows are never
	// touched.
	sql := fmt.Sprintf(`
		-- sqlstreams: consumejanitor.sweepStreamWaitingDeclarations
		WITH newest_waiting AS (
			SELECT consumer_group_id, declared_by, max(id) AS newest_id
			FROM %[1]s.%[2]s
			WHERE status = 'waiting'
			GROUP BY consumer_group_id, declared_by
		)
		DELETE FROM %[1]s.%[2]s
		WHERE id IN (
			SELECT binding_config_log.id
			FROM %[1]s.%[2]s binding_config_log
			JOIN newest_waiting ON newest_waiting.consumer_group_id = binding_config_log.consumer_group_id
				AND newest_waiting.declared_by = binding_config_log.declared_by
			WHERE binding_config_log.status = 'waiting'
			AND binding_config_log.attempted_at < $1
			AND binding_config_log.id < newest_waiting.newest_id
			LIMIT $2
		);
	`, d.Datastore.Schema, stream.BindingConfigLogTable(streamId))
	tag, err := d.Datastore.Pool.Exec(ctx, sql, cutoff, batchSize)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// listGroupStreamIds is every stream id with registered groups. A binding_config_log
// row cascades with its group, so these streams cover every declaration.
func (d *JanitorDatastore) listGroupStreamIds(ctx context.Context) ([]int64, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: consumejanitor.listGroupStreamIds
		SELECT DISTINCT stream_id
		FROM %[1]s.consumer_group_config
		ORDER BY stream_id;
	`, d.Datastore.Schema)
	rows, err := d.Datastore.Pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streamIds []int64
	for rows.Next() {
		var streamId int64
		if err := rows.Scan(&streamId); err != nil {
			return nil, err
		}
		streamIds = append(streamIds, streamId)
	}
	return streamIds, rows.Err()
}
