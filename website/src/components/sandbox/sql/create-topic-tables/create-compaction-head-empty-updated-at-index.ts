// verbatim from pkg/topic/controller/datastore/tables.go createTopicTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { compactionHeadTable } from '../table-names';

export const createCompactionHeadEmptyUpdatedAtIndexSqlTemplate = `
		-- vulkan: topic.createTopicTables
		CREATE INDEX IF NOT EXISTS %[2]s_empty_updated_at ON %[1]s.%[2]s (updated_at, compaction_key)
			WHERE message_id IS NULL;
	`;

export function createCompactionHeadEmptyUpdatedAtIndexSql(topicId: number): string {
	return interpolate(
		createCompactionHeadEmptyUpdatedAtIndexSqlTemplate,
		compactionHeadTable(topicId),
	);
}
