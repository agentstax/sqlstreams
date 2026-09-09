package datastore

import (
	"context"
	"fmt"

	"github.com/agentstax/sqlstreams/pkg/consume"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// Kill marks expired 'inflight' rows that are out of attempts 'dead' so
// nothing else resolves them. Returns how many rows it marked.
func (d *ExceptionConsumerGroupDatastore) Kill(ctx context.Context, streamId int64, groupId int64, maxRetries int, deliveryLogMode stream.DeliveryLogMode) (int64, error) {
	var killed int64
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		killed, err = d.kill(ctx, streamId, groupId, maxRetries, deliveryLogMode)
		return err
	})
	return killed, err
}

func (d *ExceptionConsumerGroupDatastore) kill(ctx context.Context, streamId int64, groupId int64, maxRetries int, deliveryLogMode stream.DeliveryLogMode) (int64, error) {
	var killSql string
	if deliveryLogMode == stream.DeliveryLogModeOff {
		killSql = fmt.Sprintf(`
			-- sqlstreams: exceptionconsumer.kill
			UPDATE %[1]s.%[2]s
			SET
				status = 'dead',
				lease_token = NULL,
				lease_expires_at = NULL,
				updated_at = now(),
				last_error = concat(last_error, ' [killed: crash-loop hit max attempts]')
			WHERE consumer_group_id = $1
				AND status = 'inflight'
				AND lease_expires_at < now()
				AND attempts - delays >= $2;
		`, d.Datastore.Schema, stream.ExceptionQueueTable(streamId))
	} else {
		// killed CTE + INSERT keeps the kill and its delivery_log_<stream_id> row
		// atomic in one statement.
		killSql = fmt.Sprintf(`
			-- sqlstreams: exceptionconsumer.kill
			WITH killed AS (
				UPDATE %[1]s.%[2]s
				SET
					status = 'dead',
					lease_token = NULL,
					lease_expires_at = NULL,
					updated_at = now(),
					last_error = concat(last_error, ' [killed: crash-loop hit max attempts]')
				WHERE consumer_group_id = $1
					AND status = 'inflight'
					AND lease_expires_at < now()
					AND attempts - delays >= $2
				RETURNING consumer_group_id, message_id, attempts, last_error
			)
			INSERT INTO %[1]s.%[3]s (consumer_group_id, message_id, attempt, status, error)
			SELECT consumer_group_id, message_id, attempts, 'killed', last_error
			FROM killed;
		`, d.Datastore.Schema, stream.ExceptionQueueTable(streamId), stream.DeliveryLogTable(streamId))
	}
	killTag, err := d.Datastore.Pool.Exec(ctx, killSql, groupId, maxRetries)
	if err != nil {
		return 0, err
	}
	if killTag.RowsAffected() > 0 {
		d.Logger.WarnContext(ctx, consume.EventKillBackstopFired.Message(), "code", consume.EventKillBackstopFired.GetCode(), "group_id", groupId, "stream_id", streamId, "dead_count", killTag.RowsAffected())
	}
	return killTag.RowsAffected(), nil
}
