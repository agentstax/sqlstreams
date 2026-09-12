package datastore

import (
	"context"
	"errors"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Get resolves a stream by name. Returns (nil, nil) if name is not found.
func (d *StreamDatastore) Get(ctx context.Context, name string) (*StreamConfigRow, error) {
	var streamConfigRow *StreamConfigRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		streamConfigRow, err = d.get(ctx, d.Datastore.Pool, name)
		return err
	})
	return streamConfigRow, err
}

// GetInTx resolves a stream by name through tx. Returns (nil, nil) if name is
// not found.
func (d *StreamDatastore) GetInTx(ctx context.Context, tx datastore.Tx, name string) (*StreamConfigRow, error) {
	return d.get(ctx, tx, name)
}

func (d *StreamDatastore) get(ctx context.Context, q datastore.Querier, name string) (*StreamConfigRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: stream.get
		SELECT
			id,
			system_id,
			name,
			partition_size,
			retention_ttl_ns,
			allow_drop_past_committed,
			idempotency_key_ttl_ns,
			empty_compaction_head_ttl_ns,
			delivery_log_mode,
			created_at,
			updated_at
		FROM %[1]s.stream_config
		WHERE name = $1;
	`, d.Datastore.Schema)
	return d.scanStreamConfigRow(q.QueryRow(ctx, sql, name))
}

// GetById resolves a stream by its id. Returns (nil, nil) if no stream has it.
func (d *StreamDatastore) GetById(ctx context.Context, id int64) (*StreamConfigRow, error) {
	var streamConfigRow *StreamConfigRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		streamConfigRow, err = d.getById(ctx, id)
		return err
	})
	return streamConfigRow, err
}

func (d *StreamDatastore) getById(ctx context.Context, id int64) (*StreamConfigRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: stream.getById
		SELECT
			id,
			system_id,
			name,
			partition_size,
			retention_ttl_ns,
			allow_drop_past_committed,
			idempotency_key_ttl_ns,
			empty_compaction_head_ttl_ns,
			delivery_log_mode,
			created_at,
			updated_at
		FROM %[1]s.stream_config
		WHERE id = $1;
	`, d.Datastore.Schema)
	return d.scanStreamConfigRow(d.Datastore.Pool.QueryRow(ctx, sql, id))
}

func (d *StreamDatastore) List(ctx context.Context) ([]StreamConfigRow, error) {
	var streams []StreamConfigRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		streams, err = d.list(ctx)
		return err
	})
	return streams, err
}

func (d *StreamDatastore) list(ctx context.Context) ([]StreamConfigRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: stream.list
		SELECT
			id,
			system_id,
			name,
			partition_size,
			retention_ttl_ns,
			allow_drop_past_committed,
			idempotency_key_ttl_ns,
			empty_compaction_head_ttl_ns,
			delivery_log_mode,
			created_at,
			updated_at
		FROM %[1]s.stream_config
		ORDER BY name;
	`, d.Datastore.Schema)
	rows, err := d.Datastore.Pool.Query(ctx, sql)
	if err != nil {
		// 42P01 = table does not exist -- an unregistered database has no streams
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var streams []StreamConfigRow
	for rows.Next() {
		streamConfigRow, err := d.scanStreamConfigRow(rows)
		if err != nil {
			return nil, err
		}
		streams = append(streams, *streamConfigRow)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return streams, nil
}

// Register resolves declared's name to its row, creating
// it (and its per-stream tables) if it doesn't exist. An existing row takes
// declared's mutable config; its partition_size must match.
func (d *StreamDatastore) Register(ctx context.Context, declared *StreamConfigRow, declaredBy string) (*StreamConfigRow, error) {
	var registered *StreamConfigRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		registered, err = d.register(ctx, declared, declaredBy)
		return err
	})
	return registered, err
}

// register registers behind a per-name advisory lock, NOT ON CONFLICT.
// This is to prevent race condition errors between two concurrent calls.
func (d *StreamDatastore) register(ctx context.Context, declared *StreamConfigRow, declaredBy string) (*StreamConfigRow, error) {
	// private get, not Get -- otherwise would have nested retries.
	found, err := d.get(ctx, d.Datastore.Pool, declared.Name)
	if err != nil {
		return nil, err
	}
	if found != nil {
		return d.replaceConfig(ctx, found, declared, declaredBy)
	}

	tx, err := d.Datastore.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	lockKey, err := common.NewAdvisoryLockKey("stream", d.Datastore.Schema, declared.Name)
	if err != nil {
		return nil, err
	}

	// txn-scoped, per-name -- auto-released at commit/rollback
	if _, err := tx.Exec(ctx, `
		-- sqlstreams: stream.register
		SELECT pg_advisory_xact_lock($1);
	`, lockKey.Value()); err != nil {
		return nil, err
	}

	// re-check under the lock -- a racing register may have committed while we waited
	found, err = d.get(ctx, tx, declared.Name)
	if err != nil {
		return nil, err
	}
	if found != nil {
		return d.replaceConfig(ctx, found, declared, declaredBy)
	}

	insertSql := fmt.Sprintf(`
		-- sqlstreams: stream.register
		INSERT INTO %[1]s.stream_config (system_id, name, partition_size, retention_ttl_ns, allow_drop_past_committed, idempotency_key_ttl_ns, empty_compaction_head_ttl_ns, delivery_log_mode)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at;
	`, d.Datastore.Schema)
	created := *declared
	if err := tx.QueryRow(ctx, insertSql, declared.SystemId, declared.Name, declared.PartitionSize, declared.RetentionTTLNs, declared.AllowDropPastCommitted, declared.IdempotencyKeyTTLNs, declared.EmptyCompactionHeadTTLNs, declared.DeliveryLogMode).
		Scan(&created.Id, &created.CreatedAt, &created.UpdatedAt); err != nil {
		return nil, err
	}

	if err := d.appendStreamConfigLog(ctx, tx, &created, declaredBy); err != nil {
		return nil, err
	}

	if err := d.createStreamTables(ctx, tx, created.Id, declared.PartitionSize); err != nil {
		return nil, err
	}

	// add the migration baseline in the SAME txn
	migrationSql := fmt.Sprintf(`
		-- sqlstreams: stream.register
		INSERT INTO %[1]s.migration_log (stream_id, version, status)
		VALUES ($1, 1, 'success');
	`, d.Datastore.Schema)
	if _, err := tx.Exec(ctx, migrationSql, created.Id); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	d.Logger.InfoContext(ctx, "stream registered (created)", "stream", created.Name, "stream_id", created.Id)
	return &created, nil
}

// Rename moves the stream under oldName to newName, appending its
// stream_config_log row beside the update.
// Returns (nil, nil) if no stream is registered under oldName
// ErrStreamNameTaken if newName is already registered.
func (d *StreamDatastore) Rename(ctx context.Context, oldName string, newName string, declaredBy string) (*StreamConfigRow, error) {
	var renamed *StreamConfigRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		renamed, err = d.rename(ctx, oldName, newName, declaredBy)
		return err
	})
	return renamed, err
}

func (d *StreamDatastore) rename(ctx context.Context, oldName string, newName string, declaredBy string) (*StreamConfigRow, error) {
	tx, err := d.Datastore.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	sql := fmt.Sprintf(`
		-- sqlstreams: stream.rename
		UPDATE %[1]s.stream_config
		SET name = $2, updated_at = NOW()
		WHERE name = $1
		RETURNING
			id,
			system_id,
			name,
			partition_size,
			retention_ttl_ns,
			allow_drop_past_committed,
			idempotency_key_ttl_ns,
			empty_compaction_head_ttl_ns,
			delivery_log_mode,
			created_at,
			updated_at;
	`, d.Datastore.Schema)
	renamed, err := d.scanStreamConfigRow(tx.QueryRow(ctx, sql, oldName, newName))
	if err != nil {
		// 23505 = unique constraint violation ie name taken
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, stream.ErrStreamNameTaken.With("stream", newName)
		}
		return nil, err
	}
	if renamed == nil {
		return nil, nil
	}

	if err := d.appendStreamConfigLog(ctx, tx, renamed, declaredBy); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	d.Logger.InfoContext(ctx, "stream renamed", "stream", oldName, "new_name", newName, "stream_id", renamed.Id)
	return renamed, nil
}

// appendStreamConfigLog writes data's full snapshot as one stream_config_log row, inside
// the transaction that changed the stream row.
func (d *StreamDatastore) appendStreamConfigLog(ctx context.Context, q datastore.Querier, data *StreamConfigRow, declaredBy string) error {
	sql := fmt.Sprintf(`
		-- sqlstreams: stream.appendStreamConfigLog
		INSERT INTO %[1]s.stream_config_log (stream_id, name, partition_size, retention_ttl_ns, allow_drop_past_committed, idempotency_key_ttl_ns, empty_compaction_head_ttl_ns, delivery_log_mode, declared_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
	`, d.Datastore.Schema)
	_, err := q.Exec(ctx, sql, data.Id, data.Name, data.PartitionSize, data.RetentionTTLNs, data.AllowDropPastCommitted, data.IdempotencyKeyTTLNs, data.EmptyCompactionHeadTTLNs, data.DeliveryLogMode, declaredBy)
	return err
}

// scanStreamConfigRow scans a row shaped like getStream's SELECT -- the column list
// every one of those queries shares. Returns (nil, nil) when the row -- or
// stream_config itself, 42P01 -- isn't there yet.
func (d *StreamDatastore) scanStreamConfigRow(row pgx.Row) (*StreamConfigRow, error) {
	var data StreamConfigRow
	err := row.Scan(
		&data.Id,
		&data.SystemId,
		&data.Name,
		&data.PartitionSize,
		&data.RetentionTTLNs,
		&data.AllowDropPastCommitted,
		&data.IdempotencyKeyTTLNs,
		&data.EmptyCompactionHeadTTLNs,
		&data.DeliveryLogMode,
		&data.CreatedAt,
		&data.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		// 42P01 = table does not exist
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
			return nil, nil
		}
		return nil, err
	}
	return &data, nil
}
