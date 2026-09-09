// verbatim from pkg/consume/messageconsumer/controller/datastore/claim.go
// readClaimSnapshot -- the template is drift-checked byte-exact; the
// function mirrors the fmt.Sprintf call
import { interpolate } from './interpolate';
import { claimLeaseTable, consumerGroupCursorTable, messageLogTable } from './table-names';

export const claimSnapshotSqlTemplate = `
		-- sqlstreams: messageconsumer.readClaimSnapshot
		SELECT
			h.head,
			CASE WHEN h.head = c.pending_head AND c.pending_head = c.settled_head AND c.claimed = c.settled_head
				THEN '0' ELSE pg_current_xact_id()::text END AS xmax,
			c.claimed,
			c.settled_head,
			c.pending_head,
			EXISTS (
				SELECT 1 FROM %[1]s.%[4]s l
				WHERE l.consumer_group_id = $1
					AND l.expires_at < now()
			) AS reclaimable
		FROM %[1]s.%[3]s c
		CROSS JOIN (SELECT COALESCE(MAX(id), 0) AS head FROM %[1]s.%[2]s) h
		WHERE c.consumer_group_id = $1;
	`;

export function claimSnapshotSql(streamId: number): string {
	return interpolate(
		claimSnapshotSqlTemplate,
		messageLogTable(streamId),
		consumerGroupCursorTable(streamId),
		claimLeaseTable(streamId),
	);
}
