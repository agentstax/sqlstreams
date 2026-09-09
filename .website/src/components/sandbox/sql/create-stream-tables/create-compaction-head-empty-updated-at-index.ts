// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { compactionHeadTable } from '../table-names';

export const createCompactionHeadEmptyUpdatedAtIndexSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE INDEX IF NOT EXISTS %[2]s_updated_at ON %[1]s.%[2]s (updated_at, compaction_key)
			WHERE message_id IS NULL;
	`;

export function createCompactionHeadEmptyUpdatedAtIndexSql(streamId: number): string {
	return interpolate(
		createCompactionHeadEmptyUpdatedAtIndexSqlTemplate,
		compactionHeadTable(streamId),
	);
}
