package datastore

import (
	"context"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// IsEmpty reports whether the stream's log holds any row at all.
func (d *StreamDatastore) IsEmpty(ctx context.Context, streamId int64) (bool, error) {
	var empty bool
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		empty, err = d.isEmpty(ctx, streamId)
		return err
	})
	return empty, err
}

func (d *StreamDatastore) isEmpty(ctx context.Context, streamId int64) (bool, error) {
	// Partition-pruned and LIMIT 1'd, so it stays cheap regardless of stream size.
	sql := fmt.Sprintf(`
		-- sqlstreams: stream.isEmpty
		SELECT EXISTS (SELECT 1 FROM %[1]s.%[2]s LIMIT 1);
	`, d.Datastore.Schema, stream.MessageLogTable(streamId))
	var notEmpty bool
	if err := d.Datastore.Pool.QueryRow(ctx, sql).Scan(&notEmpty); err != nil {
		return false, err
	}
	return !notEmpty, nil
}

func (d *StreamDatastore) Delete(ctx context.Context, streamId int64, name string) error {
	return d.DatastoreRetry.Wrap(ctx, func() error {
		return d.delete(ctx, streamId, name)
	})
}

func (d *StreamDatastore) delete(ctx context.Context, streamId int64, name string) error {
	if err := d.drainPartitions(ctx, streamId); err != nil {
		return err
	}

	tx, err := d.Datastore.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	configSql := fmt.Sprintf(`
		-- sqlstreams: stream.delete
		DELETE FROM %[1]s.stream_config WHERE id = $1;
	`, d.Datastore.Schema)
	if _, err := tx.Exec(ctx, configSql, streamId); err != nil {
		return err
	}

	// the now-empty message_log parent and every other per-stream table
	for _, table := range []string{
		stream.MessageLogTable(streamId),
		stream.ExceptionQueueTable(streamId),
		stream.DeliveryLogTable(streamId),
		stream.IdempotencyKeyTable(streamId),
		stream.ConsumerGroupCursorTable(streamId),
		stream.ClaimLeaseTable(streamId),
		stream.MessageKeyLeaseTable(streamId),
		stream.CompactionHeadTable(streamId),
		stream.BindingConfigTable(streamId),
		stream.BindingConfigLogTable(streamId),
	} {
		dropSql := fmt.Sprintf(`
			-- sqlstreams: stream.delete
			DROP TABLE IF EXISTS %[1]s.%[2]s;`, d.Datastore.Schema, table)
		if _, err := tx.Exec(ctx, dropSql); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	d.Logger.InfoContext(ctx, "stream destroyed", "stream", name, "stream_id", streamId)
	return nil
}
