package datastore

import (
	"context"
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// SweepExpiredEmptyCompactionHeads drains idle compaction-head rows that do
// not point at a message.
func (d *JanitorDatastore) SweepExpiredEmptyCompactionHeads(ctx context.Context, streamId int64, ttl time.Duration, batchSize int) error {
	return d.DatastoreRetry.Wrap(ctx, func() error {
		return d.sweepExpiredEmptyCompactionHeads(ctx, streamId, ttl, batchSize)
	})
}

func (d *JanitorDatastore) sweepExpiredEmptyCompactionHeads(ctx context.Context, streamId int64, ttl time.Duration, batchSize int) error {
	cutoff := time.Now().Add(-ttl)

	// Protect against an accidental infinite drain while still allowing one
	// janitor pass to catch up a bounded backlog.
	const maxEmptyCompactionHeadSweepBatches = 1000
	for range maxEmptyCompactionHeadSweepBatches {
		swept, err := d.sweepEmptyCompactionHeadsBatch(ctx, streamId, cutoff, batchSize)
		if err != nil {
			return err
		}
		if swept < batchSize {
			break
		}
	}
	return nil
}

// sweepEmptyCompactionHeadsBatch locks and deletes up to batchSize eligible
// rows. SKIP LOCKED leaves a key participating in an active transaction for a
// later pass instead of waiting on it.
func (d *JanitorDatastore) sweepEmptyCompactionHeadsBatch(ctx context.Context, streamId int64, cutoff time.Time, batchSize int) (int, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: streamjanitor.sweepEmptyCompactionHeadsBatch
		WITH expired AS (
			SELECT compaction_key
			FROM %[1]s.%[2]s
			WHERE message_id IS NULL
				AND updated_at < $1
			ORDER BY updated_at ASC, compaction_key ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		DELETE FROM %[1]s.%[2]s AS h
		USING expired
		WHERE h.compaction_key = expired.compaction_key;
	`, d.Datastore.Schema, stream.CompactionHeadTable(streamId))

	tag, err := d.Datastore.Pool.Exec(ctx, sql, cutoff, batchSize)
	if err != nil {
		return 0, err
	}

	swept := int(tag.RowsAffected())
	if swept > 0 {
		d.Logger.DebugContext(ctx, "swept expired empty compaction heads", "stream_id", streamId, "swept_count", swept, "batch_size", batchSize)
	}
	return swept, nil
}
