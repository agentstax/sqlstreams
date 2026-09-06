// verbatim from pkg/topic/controller/datastore/tables.go createTopicTables -- the
// template is drift-checked byte-exact; the function mirrors the fmt.Sprintf call
import { interpolate } from '../interpolate';
import { compactionHeadTable } from '../table-names';

export const createCompactionHeadSqlTemplate = `
		-- vulkan: topic.createTopicTables
		CREATE TABLE IF NOT EXISTS %[1]s.%[2]s (
			compaction_key  TEXT   NOT NULL PRIMARY KEY,
			head_id         BIGINT,                    -- NULL while the key has a lockable row but no winning message
			schema_version  BIGINT,                    -- the winner's payload version; compared before rank
			compaction_rank BIGINT,                    -- the winner's rank
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (num_nonnulls(head_id, schema_version, compaction_rank) IN (0, 3)) -- only all NULL rows or filled rows, no in-between
		);
	`;

export function createCompactionHeadSql(topicId: number): string {
	return interpolate(createCompactionHeadSqlTemplate, compactionHeadTable(topicId));
}
