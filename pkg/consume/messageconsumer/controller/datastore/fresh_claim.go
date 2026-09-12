package datastore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
)

// snapshot pairs the visible head with its observation transaction id,
// read by readClaimSnapshot in an earlier statement than this transaction.
func (d *MessageConsumerGroupDatastore) freshClaimMessagesWithCursor(ctx context.Context, streamId int64, groupId int64, schemaVersion int64, limit int, leaseDuration time.Duration, snapshot ClaimSnapshotRow) (*ClaimedRange, error) {
	tx, err := d.Datastore.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	cursorSql := fmt.Sprintf(`
		-- sqlstreams: messageconsumer.freshClaimMessagesWithCursor
		WITH old_values AS ( -- PG18+ has old / new syntax in returning but we want older version compatibility so use CTE
			SELECT
				claimed,
				settled_head,
				pending_head,
				pending_xid
			FROM %[1]s.%[2]s
			WHERE consumer_group_id = $1
			-- must FOR UPDATE, get race if using a basic snapshot read
			-- two same-group workers racing on one cursor row (claimed=0, head=200, limit=100):
			--
			--   worker A: claims (0, 100], txn still open
			--   worker B: takes its snapshot (claimed=0), blocks on A's row lock
			--   worker A: commits
			--   worker B: unblocks; its UPDATE re-checks the row's LATEST version
			--             (claimed=100), so high is correct: 100+100 = 200
			--
			-- but B's low comes from THIS select, and forks on its read mode:
			--
			--   snapshot read:  low = 0   (stale)  -> B returns (0, 200]   -> overlaps A
			--   FOR UPDATE:     low = 100 (latest) -> B returns (100, 200] -> disjoint
			FOR UPDATE
		),
		gate AS (
			-- The gist of this CTE is to find the highest message id (head)
			-- we can safely claim to without skipping messages from producers
			-- who haven't finished committing or aborting their transactions yet.
			--
			-- We associate each head we compare with a transaction id (xid).
			-- We then check xmin >= xid. That tells us all transactions with
			-- ids below xid have finished, so their messages have either
			-- committed or been rolled back.
			SELECT (
				SELECT MAX(pair.head)
				FROM (VALUES
					(o.settled_head, NULL::xid8),    -- already proven, no fence to pass
					($3::bigint, $4::xid8),          -- the fresh observation: snapshot.Head, snapshot.Xid
					(o.pending_head, o.pending_xid) -- the stored old pair
				) AS pair(head, xid)
				-- a pair is proven once xmin has passed its xid
				WHERE pair.xid IS NULL
					OR pg_snapshot_xmin(pg_current_snapshot()) >= pair.xid
			) AS head
			FROM old_values o
		),
		updated AS (
			UPDATE %[1]s.%[3]s c
			SET
				-- advance by up to batchLimit, capped at the proven head.
				claimed = LEAST(c.claimed + $2, gate.head),
				-- cache this poll's proof: a later poll where neither pair
				-- proves claims up to this instead.
				settled_head = gate.head,
				-- store the fresh pair for the next poll: ideally its txns will
				-- have finished by then, making it the next provable head.
				-- GREATEST so a racing peer's older pair can't overwrite a newer one
				pending_head = GREATEST(c.pending_head, $3),
				pending_xid = GREATEST(c.pending_xid, $4::xid8) -- also skips the initial NULL
			FROM old_values, gate
			WHERE c.consumer_group_id = $1
			RETURNING
				old_values.claimed AS low,
				c.claimed AS high
		)
		-- updated always fires when the cursor row exists (the pending columns
		-- store unconditionally), so:
		--
		--   state                        | rows   low    high   meaning
		--   claimed=100, proven=200      | 1      100    200    claim (100, 200]
		--   claimed=200, proven=200      | 1      200    200    caught up (low = high)
		--   no cursor row                | 0      -      -      row deleted since the
		--                                                       snapshot read it -> error
		--
		SELECT u.low, u.high FROM updated u;
	`, d.Datastore.Schema, stream.ConsumerGroupCursorTable(streamId), stream.ConsumerGroupCursorTable(streamId))
	cursorRows, err := tx.Query(ctx, cursorSql, groupId, limit, snapshot.Head, snapshot.Xid)
	if err != nil {
		return nil, err
	}

	claimedRange, err := pgx.CollectOneRow(cursorRows, pgx.RowToStructByName[ConsumerGroupCursorRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// if we didnt error a consumer with no cursor row would otherwise
			// poll forever looking caught up while messages accumulate
			return nil, fmt.Errorf("no cursor for group %d on stream %d -- was Register called?", groupId, streamId)
		}

		return nil, err
	}

	// at the proven head of message_log ie no messages to process
	if claimedRange.Low == claimedRange.High {
		// The next poll needs the observation even when this poll cannot advance.
		return nil, tx.Commit(ctx)
	}

	return d.claimMessages(ctx, tx, streamId, groupId, schemaVersion, claimedRange.Low, claimedRange.High, leaseDuration)
}

// low and high come from the cursor statement above, never from a caller --
// this guard catches a cursor row that went backwards, not bad input.
func (d *MessageConsumerGroupDatastore) claimMessages(ctx context.Context, tx pgx.Tx, streamId int64, groupId int64, schemaVersion int64, low int64, high int64, leaseDuration time.Duration) (*ClaimedRange, error) {
	if low >= high {
		return nil, fmt.Errorf("claimed range must advance: low %d, high %d", low, high)
	}

	// get new lease associated with range
	leaseSql := fmt.Sprintf(`
		-- sqlstreams: messageconsumer.claimMessages
		INSERT INTO %[1]s.%[2]s (consumer_group_id, low, high, expires_at)
		VALUES (
			$1,
			$2,
			$3,
			now() + make_interval(secs => $4)
		)
		RETURNING
			token,
			consumer_group_id,
			low,
			high,
			expires_at,
			reclaims;
	`, d.Datastore.Schema, stream.ClaimLeaseTable(streamId))
	leaseRows, err := tx.Query(ctx, leaseSql, groupId, low, high, leaseDuration.Seconds())
	if err != nil {
		return nil, err
	}

	lease, err := pgx.CollectOneRow(leaseRows, pgx.RowToStructByName[ClaimLeaseRow])
	if err != nil {
		return nil, err
	}

	messages, err := d.readMessages(ctx, tx, streamId, groupId, schemaVersion, low, high)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &ClaimedRange{Lease: lease, Messages: messages}, nil
}
