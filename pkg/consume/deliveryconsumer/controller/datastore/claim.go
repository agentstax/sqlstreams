package datastore

import (
	"context"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
)

func (d *DeliveryConsumerGroupDatastore) ClaimMessagesWithLifecycle(ctx context.Context, streamId int64, groupId int64, limit int) ([]ExceptionQueueRow, error) {
	var deliveries []ExceptionQueueRow
	err := d.DatastoreRetry.WrapNonIdempotent(ctx, func() error {
		var err error
		deliveries, err = d.claimMessagesWithLifecycle(ctx, streamId, groupId, limit)
		return err
	})
	return deliveries, err
}

func (d *DeliveryConsumerGroupDatastore) claimMessagesWithLifecycle(ctx context.Context, streamId int64, groupId int64, limit int) ([]ExceptionQueueRow, error) {
	// Claim this group's own delivery rows and move them 'ready' -> 'processing' in
	// one statement, per (group, stream, message). SKIP LOCKED keeps competing
	// workers from grabbing the same row.
	//
	// delivery only stores message_id, not the payload, so we join this stream's
	// message_log back in -- the log stays immutable, all mutation lives in delivery.
	//
	// No lease here: the lifecycle path never grew crash recovery, so a
	// 'processing' row that never gets resolved (consumer crash) just sits there.
	sql := fmt.Sprintf(`
		-- sqlstreams: deliveryconsumer.claimMessagesWithLifecycle
		WITH claimed AS (
			UPDATE %[1]s.%[2]s
			SET
				status = 'processing',
				attempts = attempts + 1,
				updated_at = now()
			WHERE (consumer_group_id, message_id) IN (
				SELECT consumer_group_id, message_id FROM %[1]s.%[2]s
				WHERE consumer_group_id = $1
					AND status = 'ready'
				ORDER BY message_id
				LIMIT $2
				FOR UPDATE SKIP LOCKED
			)
			RETURNING consumer_group_id, message_id, status, attempts
		)
		SELECT
			c.consumer_group_id,
			$3::bigint AS stream_id,
			c.message_id,
			c.status,
			c.attempts,
			m.payload,
			m.options
		FROM claimed c
		JOIN %[1]s.%[3]s m ON m.id = c.message_id
		ORDER BY c.message_id;
	`, d.Datastore.Schema, stream.ExceptionQueueTable(streamId), stream.MessageLogTable(streamId))

	rows, err := d.Datastore.Pool.Query(ctx, sql, groupId, limit, streamId)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[ExceptionQueueRow])
}
