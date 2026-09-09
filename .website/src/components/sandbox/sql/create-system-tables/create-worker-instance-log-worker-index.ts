// verbatim from pkg/system/controller/datastore/tables.go createSystemTables -- drift-checked byte-exact
export const createWorkerInstanceLogWorkerIndexSql = `
			-- sqlstreams: system.createSystemTables
			CREATE INDEX IF NOT EXISTS worker_instance_log_worker_id ON %[1]s.worker_instance_log (worker_id, id);
		`;
