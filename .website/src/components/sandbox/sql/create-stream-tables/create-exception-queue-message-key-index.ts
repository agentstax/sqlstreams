// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { exceptionQueueTable } from '../table-names';

export const createExceptionQueueMessageKeyIndexSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE INDEX IF NOT EXISTS %[2]s_consumer_group_id_message_key ON %[1]s.%[3]s (consumer_group_id, message_key, message_id);
	`;

export function createExceptionQueueMessageKeyIndexSql(streamId: number): string {
	return interpolate(
		createExceptionQueueMessageKeyIndexSqlTemplate,
		exceptionQueueTable(streamId),
		exceptionQueueTable(streamId),
	);
}
