// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { deliveryLogTable } from '../table-names';

export const createDeliveryLogSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE TABLE IF NOT EXISTS %[1]s.%[2]s (
			id BIGSERIAL PRIMARY KEY,
			consumer_group_id BIGINT NOT NULL,
			message_id BIGINT NOT NULL,
			attempt INT NOT NULL,                 -- the run this event belongs to; a claim handed back at the key gate logs under the number it returned
			status TEXT NOT NULL DEFAULT 'failure',
			error TEXT NOT NULL,
			attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`;

export function createDeliveryLogSql(streamId: number): string {
	return interpolate(createDeliveryLogSqlTemplate, deliveryLogTable(streamId));
}
