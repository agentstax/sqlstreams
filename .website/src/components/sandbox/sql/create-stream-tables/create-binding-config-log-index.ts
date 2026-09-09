// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { bindingConfigLogTable } from '../table-names';

export const createBindingConfigLogIndexSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE INDEX IF NOT EXISTS %[2]s_consumer_group_id ON %[1]s.%[3]s (consumer_group_id, status, declared_by, id);
	`;

export function createBindingConfigLogIndexSql(streamId: number): string {
	return interpolate(
		createBindingConfigLogIndexSqlTemplate,
		bindingConfigLogTable(streamId),
		bindingConfigLogTable(streamId),
	);
}
