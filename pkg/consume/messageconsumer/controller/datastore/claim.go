package datastore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
)

// ClaimMessagesWithCursor tries to pick up a crashed range (an expired lease)
// and only claims fresh work from the frontier if there's nothing to reclaim --
// so crashed ranges drain first.
func (d *MessageConsumerGroupDatastore) ClaimMessagesWithCursor(ctx context.Context, streamId int64, groupId int64, schemaVersion int64, limit int, maxRangeReclaims int, leaseDuration time.Duration, deliveryLogMode stream.DeliveryLogMode) (*ClaimedRange, error) {
	var claimed *ClaimedRange
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		claimed, err = d.claimMessagesWithCursor(ctx, streamId, groupId, schemaVersion, limit, maxRangeReclaims, leaseDuration, deliveryLogMode)
		return err
	})
	return claimed, err
}

func (d *MessageConsumerGroupDatastore) claimMessagesWithCursor(ctx context.Context, streamId int64, groupId int64, schemaVersion int64, limit int, maxRangeReclaims int, leaseDuration time.Duration, deliveryLogMode stream.DeliveryLogMode) (*ClaimedRange, error) {
	snapshot, err := d.readClaimSnapshot(ctx, streamId, groupId)
	if err != nil {
		return nil, err
	}

	// the reclaim transaction only opens when the snapshot saw an expired lease
	if snapshot.Reclaimable {
		reclaimed, err := d.reclaimWithCursor(ctx, streamId, groupId, schemaVersion, maxRangeReclaims, leaseDuration, deliveryLogMode)
		if err != nil {
			return nil, err
		}
		if reclaimed != nil {
			return reclaimed, nil
		}
		// a peer reclaimed it first -> fall through to a fresh claim
	}

	// nothing new and nothing provable: this snapshot saw the head we've
	// already proven and fully claimed.
	if snapshot.Head == snapshot.PendingHead && snapshot.PendingHead == snapshot.SettledHead && snapshot.Claimed == snapshot.SettledHead {
		return nil, nil
	}

	return d.freshClaimMessagesWithCursor(ctx, streamId, groupId, schemaVersion, limit, leaseDuration, snapshot)
}

// The observation allocates its xid after taking the statement snapshot and
// finishes before the claim transaction, so all possible earlier producers have smaller xids.
func (d *MessageConsumerGroupDatastore) readClaimSnapshot(ctx context.Context, streamId int64, groupId int64) (ClaimSnapshotRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: messageconsumer.readClaimSnapshot
		SELECT
			h.head,
			CASE
			  -- for idle polling: if visible head, previously observed head, proven-safe head
				-- and claimed position ALL agree -> no need to get pg_current_xact_id()
				WHEN h.head = c.pending_head AND c.pending_head = c.settled_head AND c.claimed = c.settled_head
				THEN '0'
				ELSE pg_current_xact_id()::text END
			AS xid,
			c.claimed,
			c.settled_head,
			c.pending_head,
			EXISTS (
				SELECT 1 FROM %[1]s.%[4]s l
				WHERE l.consumer_group_id = $1
					AND l.expires_at < now()
			) AS reclaimable
		FROM %[1]s.%[3]s c
		CROSS JOIN (SELECT COALESCE(MAX(id), 0) AS head FROM %[1]s.%[2]s) h -- allow us to return current head of log
		WHERE c.consumer_group_id = $1;
	`, d.Datastore.Schema, stream.MessageLogTable(streamId), stream.ConsumerGroupCursorTable(streamId), stream.ClaimLeaseTable(streamId))
	rows, err := d.Datastore.Pool.Query(ctx, sql, groupId)
	if err != nil {
		return ClaimSnapshotRow{}, err
	}

	snapshot, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[ClaimSnapshotRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// a consumer with no cursor row would otherwise poll forever
			// looking caught up while messages accumulate
			return ClaimSnapshotRow{}, fmt.Errorf("no cursor for group %d on stream %d -- was Register called?", groupId, streamId)
		}
		return ClaimSnapshotRow{}, err
	}
	return snapshot, nil
}

// readMessages reads streamId's message_log rows in (low, high], ordered by id.
func (d *MessageConsumerGroupDatastore) readMessages(ctx context.Context, tx pgx.Tx, streamId int64, groupId int64, schemaVersion int64, low int64, high int64) ([]MessageLogRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: messageconsumer.readMessages
		SELECT
			m.id,
			m.payload,
			m.created_at,
			COALESCE(m.routing_key, '') AS routing_key,
			COALESCE(m.message_key, '') AS message_key,
			COALESCE(m.compaction_rank, 0) AS compaction_rank,
			(m.compaction_rank IS NOT NULL) AS compacted,
			m.options
		FROM %[1]s.%[2]s m
		WHERE m.id > $1
			AND m.id <= $2
			-- rows at another payload version pass under the cursor unread
			AND m.schema_version = $4
			AND (
				-- no bindings for consumer_group exists
				NOT EXISTS (
					SELECT 1 FROM %[1]s.%[3]s b
					WHERE b.consumer_group_id = $3
				)
				-- bindings for consumer_group exists and match routing_key pattern
				OR EXISTS (
					SELECT 1 FROM %[1]s.%[4]s b
					WHERE b.consumer_group_id = $3
						AND m.routing_key ~ b.pattern_regex
				)
				-- if bindings exist but our routing_key does not match any of them
				-- we do not return anything
			)
			AND (
				-- uncompacted rows (keyless or keyed) are never superseded
				m.compaction_rank IS NULL
				-- compacted rows are eligible only if they're compaction_head's
				-- current pointer for their key -- O(1) lookup, no per-row scan
				OR m.id = (
					SELECT message_id FROM %[1]s.%[5]s
					WHERE compaction_key = m.message_key
						AND message_id IS NOT NULL
				)
			)
		-- rows MUST come back in id order or a batch LIMIT could
		-- return an arbitrary subset and the cursor would advance past unread offsets
		ORDER BY m.id;
	`, d.Datastore.Schema, stream.MessageLogTable(streamId), stream.BindingConfigTable(streamId), stream.BindingConfigTable(streamId), stream.CompactionHeadTable(streamId))

	rows, err := tx.Query(ctx, sql, low, high, groupId, schemaVersion)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByName[MessageLogRow])
}
