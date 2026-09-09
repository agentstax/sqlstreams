// verbatim from pkg/system/controller/datastore/tables.go createSystemTables -- drift-checked byte-exact
export const createWorkerConfigSql = `
		-- sqlstreams: system.createSystemTables
		CREATE TABLE IF NOT EXISTS %[1]s.worker_config (
			id BIGSERIAL PRIMARY KEY,
			system_id BIGINT REFERENCES %[1]s.system_config (id) ON DELETE CASCADE,
			stream_id BIGINT REFERENCES %[1]s.stream_config (id) ON DELETE CASCADE,
			consumer_group_id BIGINT REFERENCES %[1]s.consumer_group_config (id) ON DELETE CASCADE,
			name TEXT NOT NULL,                      -- 'manager' | 'stream_janitor' | 'consumer_group_janitor' | 'cursor_advancer' | 'message_consumer' | 'delivery_consumer' | 'exception_consumer' | 'schedule_producer' | 'metrics_collector' | user-defined
			metadata JSONB NOT NULL DEFAULT '{}',    -- per-worker config, written by the declaration that creates the row
			target_instances INT NOT NULL DEFAULT 1, -- 0 = suspended, -1 = unbounded
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (num_nonnulls(system_id, stream_id, consumer_group_id) = 1),
			CHECK (target_instances >= -1)
		);
	`;
