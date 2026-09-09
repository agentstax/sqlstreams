// verbatim from pkg/system/controller/datastore/tables.go createSystemTables -- drift-checked byte-exact
export const createWorkerInstanceLogSql = `
		-- sqlstreams: system.createSystemTables
		CREATE TABLE IF NOT EXISTS %[1]s.worker_instance_log (
			id BIGSERIAL PRIMARY KEY,
			worker_instance_id BIGINT NOT NULL,
			worker_id BIGINT NOT NULL REFERENCES %[1]s.worker_config (id) ON DELETE CASCADE,
			token UUID NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			attempts INT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,                -- copied from worker_instance
			attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW() -- this successful claim or renewal
		);
	`;
