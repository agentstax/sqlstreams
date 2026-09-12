package datastore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
)

// existingPartitions lists surviving message_log_<stream_id>_<n> partition numbers.
func (d *JanitorDatastore) existingPartitions(ctx context.Context, streamId int64) ([]int64, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: streamjanitor.existingPartitions
		SELECT REPLACE(c.relname, '%[2]s_', '')::bigint AS n
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = '%[1]s.%[3]s'::regclass;
	`, d.Datastore.Schema, stream.MessageLogTable(streamId), stream.MessageLogTable(streamId))

	rows, err := d.Datastore.Pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partitions []int64
	for rows.Next() {
		var n int64
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}

		partitions = append(partitions, n)
	}

	return partitions, rows.Err()
}

// cursorFloor is the most-lagging group's committed
// offset within this stream (nil if none exist yet).
func (d *JanitorDatastore) cursorFloor(ctx context.Context, q datastore.Querier, streamId int64) (*int64, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: streamjanitor.cursorFloor
		SELECT MIN(committed)
		FROM %[1]s.%[2]s;
	`, d.Datastore.Schema, stream.ConsumerGroupCursorTable(streamId))

	var floor *int64
	err := q.QueryRow(ctx, sql).Scan(&floor)
	return floor, err
}

// partitionExpired reports whether a partition's newest row is past ttl.
func (d *JanitorDatastore) partitionExpired(ctx context.Context, streamId int64, n int64, ttl time.Duration) (bool, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: streamjanitor.partitionExpired
		SELECT created_at FROM %[1]s.%[2]s
		ORDER BY id DESC -- rides the PK index; id order approx time order, no created_at index needed
		LIMIT 1;
	`, d.Datastore.Schema, stream.MessageLogPartitionTable(streamId, n))

	var newest time.Time
	err := d.Datastore.Pool.QueryRow(ctx, sql).Scan(&newest)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // empty -- nothing to judge, so not expired
	}
	if err != nil {
		return false, err
	}

	return time.Since(newest) >= ttl, nil
}
