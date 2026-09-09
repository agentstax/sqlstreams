// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { deliveryLogTable } from '../table-names';

export const createDeliveryLogAttemptIndexSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE INDEX IF NOT EXISTS %[2]s_consumer_group_id ON %[1]s.%[3]s (consumer_group_id, message_id, attempt);
	`;

export function createDeliveryLogAttemptIndexSql(streamId: number): string {
	return interpolate(
		createDeliveryLogAttemptIndexSqlTemplate,
		deliveryLogTable(streamId),
		deliveryLogTable(streamId),
	);
}
