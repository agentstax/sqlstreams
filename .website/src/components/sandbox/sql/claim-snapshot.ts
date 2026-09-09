// verbatim from pkg/consume/messageconsumer/controller/datastore/claim.go
// readClaimSnapshot -- the template is drift-checked byte-exact; the
// function mirrors the fmt.Sprintf call
import { interpolate } from './interpolate';
import { claimLeaseTable, consumerGroupCursorTable, messageLogTable } from './table-names';

export const claimSnapshotSqlTemplate = `
		-- sqlstreams: messageconsumer.readClaimSnapshot
		SELECT
			(SELECT COALESCE(MAX(id), 0) FROM %[1]s.%[2]s) AS head,
			pg_snapshot_xmax(pg_current_snapshot())::text AS xmax,
			c.claimed,
			c.settled_head,
			c.pending_head,
			EXISTS (
				SELECT 1 FROM %[1]s.%[4]s l
				WHERE l.consumer_group_id = $1
					AND l.expires_at < now()
			) AS reclaimable
		FROM %[1]s.%[3]s c
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
