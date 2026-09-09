// verbatim from pkg/system/controller/datastore/tables.go createSystemTables -- drift-checked byte-exact
export const createWorkerInstanceLogExpiryIndexSql = `
			-- sqlstreams: system.createSystemTables
			CREATE INDEX IF NOT EXISTS worker_instance_log_expires_at ON %[1]s.worker_instance_log (expires_at);
		`;
