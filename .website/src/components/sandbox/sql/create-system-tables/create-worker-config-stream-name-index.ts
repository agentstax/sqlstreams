// verbatim from pkg/system/controller/datastore/tables.go createSystemTables -- drift-checked byte-exact
export const createWorkerConfigStreamNameIndexSql = `
			-- sqlstreams: system.createSystemTables
			CREATE UNIQUE INDEX IF NOT EXISTS worker_config_name_stream_id ON %[1]s.worker_config (name, stream_id) WHERE stream_id IS NOT NULL;
		`;
