// verbatim from pkg/system/controller/datastore/tables.go createSystemTables -- drift-checked byte-exact
export const createConsumerGroupConfigSql = `
		-- sqlstreams: system.createSystemTables
		CREATE TABLE IF NOT EXISTS %[1]s.consumer_group_config (
			id BIGSERIAL PRIMARY KEY,                                         -- what children reference
			stream_id BIGINT NOT NULL REFERENCES %[1]s.stream_config (id) ON DELETE CASCADE, -- owning stream
			name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (stream_id, name)
		);
	`;
