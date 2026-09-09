package datastore

import (
	"context"
	"fmt"

	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
)

// StreamSnapshot returns the partition count and compaction-head state together.
func (d *MetricDatastore) StreamSnapshot(ctx context.Context, streamId int64) (*StreamSnapshotRow, error) {
	var snapshot *StreamSnapshotRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		snapshot, err = d.streamSnapshot(ctx, streamId)
		return err
	})
	return snapshot, err
}

func (d *MetricDatastore) streamSnapshot(ctx context.Context, streamId int64) (*StreamSnapshotRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: metric.streamSnapshot
		SELECT
			(SELECT count(*) FROM pg_inherits WHERE inhparent = to_regclass($1)) AS partitions,
			COUNT(message_id) > 0 AS compacted,
			COUNT(*) FILTER (WHERE message_id IS NULL) AS compaction_rows_without_head,
			COALESCE(
				EXTRACT(EPOCH FROM (NOW() - MIN(updated_at) FILTER (WHERE message_id IS NULL))),
				0
			) AS oldest_compaction_row_without_head_secs
		FROM %[1]s.%[2]s;
	`, d.Datastore.Schema, stream.CompactionHeadTable(streamId))
	var snapshot StreamSnapshotRow
	parentTableName := fmt.Sprintf("%s.%s", d.Datastore.Schema, stream.MessageLogTable(streamId))
	err := d.Datastore.Pool.QueryRow(ctx, sql, parentTableName).Scan(
		&snapshot.Partitions,
		&snapshot.Compacted,
		&snapshot.CompactionRowsWithoutHead,
		&snapshot.OldestCompactionRowWithoutHeadSecs,
	)
	return &snapshot, err
}

// SchemaVersionCounts is every payload version present in the stream's log,
// with its row count and how many compaction heads point at it.
func (d *MetricDatastore) SchemaVersionCounts(ctx context.Context, streamId int64) ([]SchemaVersionCountRow, error) {
	var counts []SchemaVersionCountRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		counts, err = d.schemaVersionCounts(ctx, streamId)
		return err
	})
	return counts, err
}

func (d *MetricDatastore) schemaVersionCounts(ctx context.Context, streamId int64) ([]SchemaVersionCountRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: metric.schemaVersionCounts
		SELECT
			m.schema_version,
			count(*) AS messages,
			count(h.message_id) AS compaction_heads
		FROM %[1]s.%[2]s m
		LEFT JOIN %[1]s.%[3]s h ON h.message_id = m.id
		GROUP BY m.schema_version
		ORDER BY m.schema_version;
	`, d.Datastore.Schema, stream.MessageLogTable(streamId), stream.CompactionHeadTable(streamId))
	rows, err := d.Datastore.Pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[SchemaVersionCountRow])
}

// ConsumerGroupSchemaVersionLag is, per consumer group on the stream, how many rows at
// schemaVersion sit above the group's committed cursor and how many of its
// exception rows at that version are unresolved.
func (d *MetricDatastore) ConsumerGroupSchemaVersionLag(ctx context.Context, streamId int64, schemaVersion int64) ([]ConsumerGroupSchemaVersionLagRow, error) {
	var lags []ConsumerGroupSchemaVersionLagRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		lags, err = d.groupSchemaVersionLag(ctx, streamId, schemaVersion)
		return err
	})
	return lags, err
}

func (d *MetricDatastore) groupSchemaVersionLag(ctx context.Context, streamId int64, schemaVersion int64) ([]ConsumerGroupSchemaVersionLagRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: metric.groupSchemaVersionLag
		SELECT
			g.name AS consumer_group,
			(
				SELECT count(*) FROM %[1]s.%[3]s m
				WHERE m.schema_version = $1
					AND m.id > c.committed
			) AS unconsumed,
			(
				SELECT count(*) FROM %[1]s.%[4]s e
				JOIN %[1]s.%[3]s m ON m.id = e.message_id
				WHERE e.consumer_group_id = c.consumer_group_id
					AND m.schema_version = $1
					AND e.status IN ('ready', 'inflight', 'deferred')
			) AS unresolved_exceptions
		FROM %[1]s.%[2]s c
		JOIN %[1]s.consumer_group_config g ON g.id = c.consumer_group_id
		ORDER BY g.name;
	`, d.Datastore.Schema, stream.ConsumerGroupCursorTable(streamId), stream.MessageLogTable(streamId), stream.ExceptionQueueTable(streamId))
	rows, err := d.Datastore.Pool.Query(ctx, sql, schemaVersion)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[ConsumerGroupSchemaVersionLagRow])
}
