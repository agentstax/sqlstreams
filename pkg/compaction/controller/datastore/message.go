package datastore

import (
	"context"
	"fmt"

	"github.com/agentstax/vulkan/pkg/topic"
	"github.com/jackc/pgx/v5"
)

// ListKeyMessages reads messageKey's retained messages, newest first.
func (d *CompactionDatastore) ListKeyMessages(ctx context.Context, topicId int64, messageKey string, limit int) ([]MessageLogRow, error) {
	var messages []MessageLogRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		messages, err = d.listKeyMessages(ctx, topicId, messageKey, limit)
		return err
	})
	return messages, err
}

func (d *CompactionDatastore) listKeyMessages(ctx context.Context, topicId int64, messageKey string, limit int) ([]MessageLogRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: compaction.listKeyMessages
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
	`, d.Datastore.Schema, topic.MessageLogTable(topicId))

	rows, err := d.Datastore.Pool.Query(ctx, sql, messageKey, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return d.scanMessageLogRows(rows)
}

// ListKeyMessagesByRank reads the inclusive rank interval without a row limit,
// ordered by rank then id descending. Uncompacted messages are excluded.
func (d *CompactionDatastore) ListKeyMessagesByRank(ctx context.Context, topicId int64, messageKey string, minimumRank int64, maximumRank int64) ([]MessageLogRow, error) {
	var messages []MessageLogRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		messages, err = d.listKeyMessagesByRank(ctx, topicId, messageKey, minimumRank, maximumRank)
		return err
	})
	return messages, err
}

func (d *CompactionDatastore) listKeyMessagesByRank(ctx context.Context, topicId int64, messageKey string, minimumRank int64, maximumRank int64) ([]MessageLogRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: compaction.listKeyMessagesByRank
		SELECT
			id,
			payload,
			created_at,
			COALESCE(routing_key, ''),
			message_key,
			COALESCE(compaction_rank, 0)
		FROM %[1]s.%[2]s
		WHERE message_key = $1
			AND compaction_rank BETWEEN $2 AND $3
		ORDER BY compaction_rank DESC, id DESC;
	`, d.Datastore.Schema, topic.MessageLogTable(topicId))

	rows, err := d.Datastore.Pool.Query(ctx, sql, messageKey, minimumRank, maximumRank)
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
