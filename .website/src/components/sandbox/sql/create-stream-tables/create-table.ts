// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { messageLogTable } from '../table-names';

export const createTableSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE TABLE IF NOT EXISTS %[1]s.%[2]s (
			id BIGSERIAL PRIMARY KEY,
			-- never ALTER SEQUENCE ... CACHE on this sequence: the consumer's
			-- claim fence assumes ids are issued in INSERT order, and a cached
			-- sequence hands out out-of-order id blocks

			schema_version INTEGER NOT NULL,              -- the payload's version, from the producing Message type
			routing_key TEXT,
			message_key TEXT,
			compaction_rank BIGINT,                       -- NULL = this message never opted into compaction
			payload JSONB NOT NULL,
			options JSONB,                                -- sparse MessageOptions
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		) PARTITION BY RANGE (id);
	`;

export function createTableSql(streamId: number): string {
	return interpolate(createTableSqlTemplate, messageLogTable(streamId));
}
