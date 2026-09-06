package datastore

import (
	"context"
	"fmt"

	"github.com/agentstax/vulkan/pkg/topic"
	"github.com/jackc/pgx/v5"
)

// TopicSnapshot returns topicId's compaction-head state in one query.
func (d *MetricsDatastore) TopicSnapshot(ctx context.Context, topicId int64) (*TopicSnapshotRow, error) {
	var snapshot *TopicSnapshotRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		snapshot, err = d.topicSnapshot(ctx, topicId)
		return err
	})
	return snapshot, err
}

func (d *MetricsDatastore) topicSnapshot(ctx context.Context, topicId int64) (*TopicSnapshotRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: metrics.topicSnapshot
		SELECT
			COUNT(message_id) > 0 AS compacted,
			COUNT(*) FILTER (WHERE message_id IS NULL) AS compaction_rows_without_head,
			COALESCE(
				EXTRACT(EPOCH FROM (NOW() - MIN(updated_at) FILTER (WHERE message_id IS NULL))),
				0
			) AS oldest_compaction_row_without_head_secs
		FROM %[1]s.%[2]s;
	`, d.Datastore.Schema, topic.CompactionHeadTable(topicId))
	var snapshot TopicSnapshotRow
	err := d.Datastore.Pool.QueryRow(ctx, sql).Scan(
		&snapshot.Compacted,
		&snapshot.CompactionRowsWithoutHead,
		&snapshot.OldestCompactionRowWithoutHeadSecs,
	)
	return &snapshot, err
}

// SchemaVersionCounts is every payload version present in the topic's log,
// with its row count and how many compaction heads point at it.
func (d *MetricsDatastore) SchemaVersionCounts(ctx context.Context, topicId int64) ([]SchemaVersionCountRow, error) {
	var counts []SchemaVersionCountRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		counts, err = d.schemaVersionCounts(ctx, topicId)
		return err
	})
	return counts, err
}

func (d *MetricsDatastore) schemaVersionCounts(ctx context.Context, topicId int64) ([]SchemaVersionCountRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: metrics.schemaVersionCounts
		SELECT
			m.schema_version,
			count(*) AS messages,
			count(h.message_id) AS compaction_heads
		FROM %[1]s.%[2]s m
		LEFT JOIN %[1]s.%[3]s h ON h.message_id = m.id
		GROUP BY m.schema_version
		ORDER BY m.schema_version;
	`, d.Datastore.Schema, topic.MessageLogTable(topicId), topic.CompactionHeadTable(topicId))
	rows, err := d.Datastore.Pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[SchemaVersionCountRow])
}

// ConsumerGroupSchemaVersionLag is, per consumer group on the topic, how many rows at
// schemaVersion sit above the group's committed cursor and how many of its
// exception rows at that version are unresolved.
func (d *MetricsDatastore) ConsumerGroupSchemaVersionLag(ctx context.Context, topicId int64, schemaVersion int64) ([]ConsumerGroupSchemaVersionLagRow, error) {
	var lags []ConsumerGroupSchemaVersionLagRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		lags, err = d.groupSchemaVersionLag(ctx, topicId, schemaVersion)
		return err
	})
	return lags, err
}

func (d *MetricsDatastore) groupSchemaVersionLag(ctx context.Context, topicId int64, schemaVersion int64) ([]ConsumerGroupSchemaVersionLagRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: metrics.groupSchemaVersionLag
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
	`, d.Datastore.Schema, topic.ConsumerGroupCursorTable(topicId), topic.MessageLogTable(topicId), topic.ExceptionQueueTable(topicId))
	rows, err := d.Datastore.Pool.Query(ctx, sql, schemaVersion)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[ConsumerGroupSchemaVersionLagRow])
}
