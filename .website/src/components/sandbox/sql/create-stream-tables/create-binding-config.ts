// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { bindingConfigTable } from '../table-names';

export const createBindingConfigSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE TABLE IF NOT EXISTS %[1]s.%[2]s (
			id BIGSERIAL PRIMARY KEY,
			consumer_group_id BIGINT NOT NULL REFERENCES %[1]s.consumer_group_config (id) ON DELETE CASCADE,
			pattern TEXT,                             -- the declared NATS-style pattern, for humans
			pattern_regex TEXT NOT NULL,              -- POSIX regex translated from the declared pattern
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (consumer_group_id, pattern_regex) -- its index also serves the group lookup
		);
	`;

export function createBindingConfigSql(streamId: number): string {
	return interpolate(createBindingConfigSqlTemplate, bindingConfigTable(streamId));
}
