package datastore

import (
	"context"
	"errors"
	"fmt"

	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/topic"
	"github.com/jackc/pgx/v5"
)

// LockHead ensures messageKey has a row and holds its row lock until the
// caller's transaction resolves. No retry wrapper belongs here: the
// caller owns the transaction and decides whether its whole closure is safe
// to run again.
func (d *CompactionDatastore) LockHead(ctx context.Context, tx iDatastore.Tx, topicId int64, messageKey string) (*MessageLogRow, error) {
	head, err := d.ensureAndLockHead(ctx, tx, topicId, messageKey)
	if err != nil {
		return nil, err
	}
	if head.HeadId == nil {
		return nil, nil
	}
	return d.getHeadMessage(ctx, tx, topicId, *head.HeadId)
}

// ensureAndLockHead creates the lockable identity when absent. On conflict,
// PostgreSQL's update locks the existing row until tx resolves; the CASE
// refreshes only an empty row's activity timestamp while still returning a
// populated row unchanged.
func (d *CompactionDatastore) ensureAndLockHead(ctx context.Context, q iDatastore.Querier, topicId int64, messageKey string) (*CompactionHeadRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: compaction.ensureAndLockHead
		INSERT INTO %[1]s.%[2]s AS h (compaction_key)
		VALUES ($1)
		ON CONFLICT (compaction_key) DO UPDATE
		SET updated_at = CASE
			WHEN h.head_id IS NULL THEN NOW()
			ELSE h.updated_at
		END
		RETURNING
			compaction_key,
			head_id,
			schema_version,
			compaction_rank,
			created_at,
			updated_at;
	`, d.Datastore.Schema, topic.CompactionHeadTable(topicId))

	var head CompactionHeadRow
	err := q.QueryRow(ctx, sql, messageKey).Scan(
		&head.CompactionKey,
		&head.HeadId,
		&head.SchemaVersion,
		&head.CompactionRank,
		&head.CreatedAt,
		&head.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &head, nil
}

func (d *CompactionDatastore) getHeadMessage(ctx context.Context, q iDatastore.Querier, topicId int64, headId int64) (*MessageLogRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: compaction.getHeadMessage
		SELECT
			id,
			payload,
			created_at,
			COALESCE(routing_key, ''),
			message_key,
			compaction_rank
		FROM %[1]s.%[2]s
		WHERE id = $1;
	`, d.Datastore.Schema, topic.MessageLogTable(topicId))

	var head MessageLogRow
	err := q.QueryRow(ctx, sql, headId).Scan(
		&head.Id,
		&head.Payload,
		&head.CreatedAt,
		&head.RoutingKey,
		&head.MessageKey,
		&head.CompactionRank,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &head, nil
}

// GetHead reads the current compaction head under messageKey,
// nil if the key has no head.
func (d *CompactionDatastore) GetHead(ctx context.Context, topicId int64, messageKey string) (*MessageLogRow, error) {
	var head *MessageLogRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		head, err = d.getHead(ctx, topicId, messageKey)
		return err
	})
	return head, err
}

func (d *CompactionDatastore) getHead(ctx context.Context, topicId int64, messageKey string) (*MessageLogRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: compaction.getHead
		SELECT
			m.id,
			m.payload,
			m.created_at,
			COALESCE(m.routing_key, ''),
			m.message_key,
			m.compaction_rank
		FROM %[1]s.%[2]s h
		JOIN %[1]s.%[3]s m ON m.id = h.head_id
		WHERE h.compaction_key = $1;
	`, d.Datastore.Schema, topic.CompactionHeadTable(topicId), topic.MessageLogTable(topicId))

	var head MessageLogRow
	err := d.Datastore.Pool.QueryRow(ctx, sql, messageKey).Scan(
		&head.Id,
		&head.Payload,
		&head.CreatedAt,
		&head.RoutingKey,
		&head.MessageKey,
		&head.CompactionRank,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &head, nil
}

// ListHeads reads every key's current head on the topic, ordered by
// message key.
func (d *CompactionDatastore) ListHeads(ctx context.Context, topicId int64) ([]MessageLogRow, error) {
	var heads []MessageLogRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		heads, err = d.listHeads(ctx, topicId)
		return err
	})
	return heads, err
}

func (d *CompactionDatastore) listHeads(ctx context.Context, topicId int64) ([]MessageLogRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: compaction.listHeads
		SELECT
			m.id,
			m.payload,
			m.created_at,
			COALESCE(m.routing_key, ''),
			m.message_key,
			m.compaction_rank
		FROM %[1]s.%[2]s h
		JOIN %[1]s.%[3]s m ON m.id = h.head_id
		ORDER BY h.compaction_key;
	`, d.Datastore.Schema, topic.CompactionHeadTable(topicId), topic.MessageLogTable(topicId))

	rows, err := d.Datastore.Pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var heads []MessageLogRow
	for rows.Next() {
		var head MessageLogRow
		if err := rows.Scan(
			&head.Id,
			&head.Payload,
			&head.CreatedAt,
			&head.RoutingKey,
			&head.MessageKey,
			&head.CompactionRank,
		); err != nil {
			return nil, err
		}
		heads = append(heads, head)
	}
	return heads, rows.Err()
}
