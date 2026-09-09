// verbatim from pkg/consume/messageconsumer/controller/datastore/fresh_claim.go
// freshClaimMessagesWithCursor -- the template is drift-checked byte-exact; the
// function mirrors the fmt.Sprintf call
import { interpolate } from './interpolate';
import { consumerGroupCursorTable } from './table-names';

export const claimCursorSqlTemplate = `
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
			-- we can safely claim without skipping messages whose producers
			-- haven't finished committing or aborting their transactions yet.
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
	`;

export function claimCursorSql(streamId: number): string {
	return interpolate(
		claimCursorSqlTemplate,
		consumerGroupCursorTable(streamId),
		consumerGroupCursorTable(streamId),
	);
}
