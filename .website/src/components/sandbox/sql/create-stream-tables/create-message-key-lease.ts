// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { messageKeyLeaseTable } from '../table-names';

export const createMessageKeyLeaseSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE TABLE IF NOT EXISTS %[1]s.%[2]s (
			consumer_group_id BIGINT NOT NULL, -- PK
			message_key TEXT NOT NULL,         -- PK
			token UUID NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (consumer_group_id, message_key)
		);
	`;

export function createMessageKeyLeaseSql(streamId: number): string {
	return interpolate(createMessageKeyLeaseSqlTemplate, messageKeyLeaseTable(streamId));
}
