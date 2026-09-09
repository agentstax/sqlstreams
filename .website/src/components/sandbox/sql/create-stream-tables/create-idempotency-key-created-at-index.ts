// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { idempotencyKeyTable } from '../table-names';

export const createIdempotencyKeyCreatedAtIndexSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE INDEX IF NOT EXISTS %[2]s_created_at ON %[1]s.%[3]s (created_at);
	`;

export function createIdempotencyKeyCreatedAtIndexSql(streamId: number): string {
	return interpolate(
		createIdempotencyKeyCreatedAtIndexSqlTemplate,
		idempotencyKeyTable(streamId),
		idempotencyKeyTable(streamId),
	);
}
