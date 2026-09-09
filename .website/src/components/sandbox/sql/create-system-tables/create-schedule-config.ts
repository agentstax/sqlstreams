// verbatim from pkg/system/controller/datastore/tables.go createSystemTables -- drift-checked byte-exact
export const createScheduleConfigSql = `
		-- sqlstreams: system.createSystemTables
		CREATE TABLE IF NOT EXISTS %[1]s.schedule_config (
			id BIGSERIAL PRIMARY KEY,
			system_id BIGINT NOT NULL REFERENCES %[1]s.system_config (id) ON DELETE CASCADE,
			stream_id BIGINT NOT NULL REFERENCES %[1]s.stream_config (id) ON DELETE CASCADE,  -- the target stream every produce lands on
			name TEXT NOT NULL UNIQUE,                       -- also the message key and routing key of every produce
			expression TEXT NOT NULL,                        -- cron expression; UTC unless it carries TZ=
			suspended BOOLEAN NOT NULL DEFAULT false,        -- a suspended schedule keeps its expression but never produces
			schema_version INTEGER NOT NULL,                 -- the payload's Message type version, written on every produce
			payload JSONB NOT NULL DEFAULT '{}',             -- the message, marshaled once at Register
			concurrency TEXT NOT NULL DEFAULT 'parallel',    -- 'parallel' | 'exclusive' -> MessageOptions.Concurrency
			timeout_ns BIGINT NOT NULL,                      -- nanoseconds; -> MessageOptions.Timeout
			metadata JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (timeout_ns > 0)
		);
	`;
