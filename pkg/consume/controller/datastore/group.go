package datastore

import (
	"context"
	"errors"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// GetGroup resolves a consumer group by its owning stream and name.
// Returns (nil, nil) if the group is not registered on that stream.
func (d *ConsumeDatastore) GetGroup(ctx context.Context, streamId int64, name string) (*ConsumerGroupConfigRow, error) {
	var group *ConsumerGroupConfigRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		group, err = d.getGroup(ctx, d.Datastore.Pool, streamId, name)
		return err
	})
	return group, err
}

func (d *ConsumeDatastore) getGroup(ctx context.Context, q datastore.Querier, streamId int64, name string) (*ConsumerGroupConfigRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: consume.getGroup
		SELECT id, stream_id, name, created_at
		FROM %[1]s.consumer_group_config
		WHERE stream_id = $1 AND name = $2;
	`, d.Datastore.Schema)
	var group ConsumerGroupConfigRow
	err := q.QueryRow(ctx, sql, streamId, name).Scan(&group.Id, &group.StreamId, &group.Name, &group.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &group, nil
}

// ListGroups lists the stream's consumer groups, ordered by name.
func (d *ConsumeDatastore) ListGroups(ctx context.Context, streamId int64) ([]ConsumerGroupConfigRow, error) {
	var groups []ConsumerGroupConfigRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		groups, err = d.listGroups(ctx, streamId)
		return err
	})
	return groups, err
}

func (d *ConsumeDatastore) listGroups(ctx context.Context, streamId int64) ([]ConsumerGroupConfigRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: consume.listGroups
		SELECT id, stream_id, name, created_at
		FROM %[1]s.consumer_group_config
		WHERE stream_id = $1
		ORDER BY name;
	`, d.Datastore.Schema)
	rows, err := d.Datastore.Pool.Query(ctx, sql, streamId)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[ConsumerGroupConfigRow])
}

// RegisterGroup registers the group and its cursor if it doesn't exist; start
// places the cursor only when this call creates the row.
func (d *ConsumeDatastore) RegisterGroup(ctx context.Context, streamId int64, name string, start consume.CursorPosition) (*ConsumerGroupConfigRow, error) {
	var group *ConsumerGroupConfigRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		group, err = d.registerGroup(ctx, streamId, name, start)
		return err
	})
	return group, err
}

// registerGroup registers behind a per-(stream,name) advisory lock, NOT ON CONFLICT.
// This is to prevent race condition errors between two concurrent calls.
func (d *ConsumeDatastore) registerGroup(ctx context.Context, streamId int64, name string, start consume.CursorPosition) (*ConsumerGroupConfigRow, error) {
	tx, err := d.Datastore.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// private getGroup, not GetGroup -- otherwise would have nested retries.
	found, err := d.getGroup(ctx, tx, streamId, name)
	if err != nil {
		return nil, err
	}
	if found != nil {
		return found, nil
	}

	lockKey, err := common.NewAdvisoryLockKey("consumer_group", d.Datastore.Schema, streamId, name)
	if err != nil {
		return nil, err
	}

	// txn-scoped, per-(stream, name) -- auto-released at commit/rollback
	if _, err := tx.Exec(ctx, `
		-- sqlstreams: consume.registerGroup
		SELECT pg_advisory_xact_lock($1);
	`, lockKey.Value()); err != nil {
		return nil, err
	}

	// re-check under the lock -- a racing registration may have committed while we waited
	found, err = d.getGroup(ctx, tx, streamId, name)
	if err != nil {
		return nil, err
	}
	if found != nil {
		return found, nil
	}

	insertSql := fmt.Sprintf(`
		-- sqlstreams: consume.registerGroup
		INSERT INTO %[1]s.consumer_group_config (stream_id, name)
		VALUES ($1, $2)
		RETURNING id, stream_id, name, created_at;
	`, d.Datastore.Schema)
	var group ConsumerGroupConfigRow
	if err := tx.QueryRow(ctx, insertSql, streamId, name).Scan(&group.Id, &group.StreamId, &group.Name, &group.CreatedAt); err != nil {
		// 23503 = the stream_id FK -- name the real problem, not the constraint
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, stream.ErrStreamNotFound.With("stream_id", streamId)
		}
		return nil, err
	}

	committed, err := d.insertCursor(ctx, tx, streamId, group.Id, start)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	d.Logger.InfoContext(ctx, "consumer group registered (created)", "group", group.Name, "stream_id", group.StreamId, "group_id", group.Id, "committed", committed)
	return &group, nil
}

// insertCursor writes the group's cursor row at the declared position and
// returns the committed id it starts from.
func (d *ConsumeDatastore) insertCursor(ctx context.Context, q datastore.Querier, streamId int64, groupId int64, start consume.CursorPosition) (int64, error) {
	var sql string
	switch start.Kind {
	case consume.CursorPositionBeginning:
		sql = fmt.Sprintf(`
			-- sqlstreams: consume.insertCursor
			INSERT INTO %[1]s.%[2]s (consumer_group_id)
			VALUES ($1)
			RETURNING committed;
		`, d.Datastore.Schema, stream.ConsumerGroupCursorTable(streamId))
	case consume.CursorPositionHead:
		sql = fmt.Sprintf(`
			-- sqlstreams: consume.insertCursor
			INSERT INTO %[1]s.%[2]s (consumer_group_id, claimed, committed, settled_head)
			SELECT $1, head, head, head
			FROM (SELECT COALESCE(MAX(id), 0) AS head FROM %[1]s.%[3]s) AS log
			RETURNING committed;
		`, d.Datastore.Schema, stream.ConsumerGroupCursorTable(streamId), stream.MessageLogTable(streamId))
	default:
		return 0, fmt.Errorf("unrecognized cursor position kind: %q", start.Kind)
	}
	var committed int64
	if err := q.QueryRow(ctx, sql, groupId).Scan(&committed); err != nil {
		return 0, err
	}
	return committed, nil
}

// DeleteGroup deletes the group and every row it owns in one transaction.
func (d *ConsumeDatastore) DeleteGroup(ctx context.Context, streamId int64, groupId int64, name string) error {
	return d.DatastoreRetry.Wrap(ctx, func() error {
		return d.deleteGroup(ctx, streamId, groupId, name)
	})
}

func (d *ConsumeDatastore) deleteGroup(ctx context.Context, streamId int64, groupId int64, name string) error {
	tx, err := d.Datastore.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// no cascade -- nothing references the per-stream claim_lease table
	leaseSql := fmt.Sprintf(`
		-- sqlstreams: consume.deleteGroup
		DELETE FROM %[1]s.%[2]s WHERE consumer_group_id = $1;
	`, d.Datastore.Schema, stream.ClaimLeaseTable(streamId))
	if _, err := tx.Exec(ctx, leaseSql, groupId); err != nil {
		return err
	}

	// no cascade -- nothing references the per-stream message_key_lease table
	keyLeaseSql := fmt.Sprintf(`
		-- sqlstreams: consume.deleteGroup
		DELETE FROM %[1]s.%[2]s WHERE consumer_group_id = $1;
	`, d.Datastore.Schema, stream.MessageKeyLeaseTable(streamId))
	if _, err := tx.Exec(ctx, keyLeaseSql, groupId); err != nil {
		return err
	}

	// no cascade -- nothing references the per-stream exception_queue table
	deliverySql := fmt.Sprintf(`
		-- sqlstreams: consume.deleteGroup
		DELETE FROM %[1]s.%[2]s WHERE consumer_group_id = $1;
	`, d.Datastore.Schema, stream.ExceptionQueueTable(streamId))
	if _, err := tx.Exec(ctx, deliverySql, groupId); err != nil {
		return err
	}

	// no cascade -- nothing references the per-stream delivery_log table
	deliveryLogSql := fmt.Sprintf(`
		-- sqlstreams: consume.deleteGroup
		DELETE FROM %[1]s.%[2]s WHERE consumer_group_id = $1;
	`, d.Datastore.Schema, stream.DeliveryLogTable(streamId))
	if _, err := tx.Exec(ctx, deliveryLogSql, groupId); err != nil {
		return err
	}

	// cascades: cursor, binding, migration_log, group-owned worker and
	// schedule rows; worker_instance follows its worker
	configSql := fmt.Sprintf(`
		-- sqlstreams: consume.deleteGroup
		DELETE FROM %[1]s.consumer_group_config WHERE id = $1;
	`, d.Datastore.Schema)
	if _, err := tx.Exec(ctx, configSql, groupId); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	d.Logger.InfoContext(ctx, "consumer group deleted", "group", name, "stream_id", streamId, "group_id", groupId)
	return nil
}
