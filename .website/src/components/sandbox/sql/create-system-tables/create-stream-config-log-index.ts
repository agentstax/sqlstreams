// verbatim from pkg/system/controller/datastore/tables.go createSystemTables -- drift-checked byte-exact
export const createStreamConfigLogIndexSql = `
		-- sqlstreams: system.createSystemTables
		CREATE INDEX IF NOT EXISTS stream_config_log_stream_id ON %[1]s.stream_config_log (stream_id, id);
	`;
