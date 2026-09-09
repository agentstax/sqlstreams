package datastore

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
)

// ListKeyMessages reads messageKey's retained messages, newest first.
func (d *CompactionDatastore) ListKeyMessages(ctx context.Context, streamId int64, messageKey string, limit int) ([]MessageLogRow, error) {
	var messages []MessageLogRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		messages, err = d.listKeyMessages(ctx, streamId, messageKey, limit)
		return err
	})
	return messages, err
}

func (d *CompactionDatastore) listKeyMessages(ctx context.Context, streamId int64, messageKey string, limit int) ([]MessageLogRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: compaction.listKeyMessages
		SELECT
			id,
			payload,
			created_at,
			COALESCE(routing_key, ''),
			message_key,
			COALESCE(compaction_rank, 0)
		FROM %[1]s.%[2]s
		WHERE message_key = $1
		ORDER BY id DESC
		LIMIT $2;
	`, d.Datastore.Schema, stream.MessageLogTable(streamId))

	rows, err := d.Datastore.Pool.Query(ctx, sql, messageKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanMessageLogRows(rows)
}

// ListKeyMessagesByCreatedAt reads the inclusive storage-time interval without
// a row limit, ordered by created_at then id descending.
func (d *CompactionDatastore) ListKeyMessagesByCreatedAt(ctx context.Context, streamId int64, messageKey string, start time.Time, end time.Time) ([]MessageLogRow, error) {
	var messages []MessageLogRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		messages, err = d.listKeyMessagesByCreatedAt(ctx, streamId, messageKey, start, end)
		return err
	})
	return messages, err
}

func (d *CompactionDatastore) listKeyMessagesByCreatedAt(ctx context.Context, streamId int64, messageKey string, start time.Time, end time.Time) ([]MessageLogRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: compaction.listKeyMessagesByCreatedAt
		SELECT
			id,
			payload,
			created_at,
			COALESCE(routing_key, ''),
			message_key,
			COALESCE(compaction_rank, 0)
		FROM %[1]s.%[2]s
		WHERE message_key = $1
			AND created_at BETWEEN $2 AND $3
		ORDER BY created_at DESC, id DESC;
	`, d.Datastore.Schema, stream.MessageLogTable(streamId))

	rows, err := d.Datastore.Pool.Query(ctx, sql, messageKey, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanMessageLogRows(rows)
}

func (d *CompactionDatastore) scanMessageLogRows(rows pgx.Rows) ([]MessageLogRow, error) {
	var messages []MessageLogRow
	for rows.Next() {
		var message MessageLogRow
		if err := rows.Scan(
			&message.Id,
			&message.Payload,
			&message.CreatedAt,
			&message.RoutingKey,
			&message.MessageKey,
			&message.CompactionRank,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}
