// verbatim from pkg/stream/controller/datastore/tables.go createStreamTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { messageLogPartitionTable, messageLogTable } from '../table-names';

export const createPartitionSqlTemplate = `
		-- sqlstreams: stream.createStreamTables
		CREATE TABLE IF NOT EXISTS %[1]s.%[2]s
			PARTITION OF %[1]s.%[3]s
			FOR VALUES FROM (0) TO (%[4]d);
	`;

export function createPartitionSql(streamId: number, partitionSize: number): string {
	return interpolate(
		createPartitionSqlTemplate,
		messageLogPartitionTable(streamId, 0),
		messageLogTable(streamId),
		partitionSize,
	);
}
