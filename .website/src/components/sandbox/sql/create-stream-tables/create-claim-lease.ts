// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { claimLeaseTable } from '../table-names';

export const createClaimLeaseSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE TABLE IF NOT EXISTS %[1]s.%[2]s (
			consumer_group_id BIGINT NOT NULL,
			token UUID NOT NULL DEFAULT gen_random_uuid(),
			low BIGINT NOT NULL,             -- low of claimed range of lease
			high BIGINT NOT NULL,            -- high of claimed range of lease
			expires_at TIMESTAMPTZ NOT NULL, -- past it the lease is reclaimed
			reclaims INT NOT NULL DEFAULT 0, -- times this range has been reclaimed; past MaxReclaims it's quarantined
			PRIMARY KEY (consumer_group_id, token)
		);
	`;

export function createClaimLeaseSql(streamId: number): string {
	return interpolate(createClaimLeaseSqlTemplate, claimLeaseTable(streamId));
}
