package checker

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// labSchema is the Postgres namespace the checker loads the ledger into, on
// the same database as vulkan's own schema so the checks are plain joins.
const labSchema = "lab"

const (
	produceTable = "produce_ledger"
	handlerTable = "handler_ledger"
	phaseTable   = "run_phase"
)

// tables is the loaded ledger: one table per file kind in labSchema, with
// the JSON-lines field names as columns. create drops and recreates the
// schema, so a checker run reads only the files it loaded.
type tables struct {
	pool *pgxpool.Pool
}

func newTables(pool *pgxpool.Pool) (*tables, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	return &tables{pool: pool}, nil
}

func (t *tables) create(ctx context.Context) error {
	createSql := fmt.Sprintf(`
		-- lab: checker.create
		DROP SCHEMA IF EXISTS %[1]s CASCADE;
		CREATE SCHEMA %[1]s;

		CREATE TABLE %[1]s.%[2]s (
			at           TIMESTAMPTZ NOT NULL,
			kind         TEXT NOT NULL,           -- 'attempted' | 'committed' | 'rejected' | 'unknown'
			producer     TEXT NOT NULL,
			seq          BIGINT NOT NULL,
			key          TEXT NOT NULL,           -- '<producer>-<seq>', the idempotency key
			scheduled_at TIMESTAMPTZ NOT NULL,
			message_id   BIGINT NOT NULL,         -- 0 unless committed
			duplicate    BOOLEAN NOT NULL,
			code         TEXT NOT NULL,           -- '' unless rejected
			error        TEXT NOT NULL            -- '' unless rejected or unknown
		);
		CREATE INDEX %[2]s_key ON %[1]s.%[2]s (key);
		CREATE INDEX %[2]s_kind_message_id ON %[1]s.%[2]s (kind, message_id);

		CREATE TABLE %[1]s.%[3]s (
			at         TIMESTAMPTZ NOT NULL,
			consumer   TEXT NOT NULL,
			"group"    TEXT NOT NULL,
			message_id BIGINT NOT NULL,
			key        TEXT NOT NULL,
			attempt    INT NOT NULL,
			outcome    TEXT NOT NULL               -- 'success' | 'error'
		);
		CREATE INDEX %[3]s_message_id ON %[1]s.%[3]s (message_id);

		CREATE TABLE %[1]s.%[4]s (
			at     TIMESTAMPTZ NOT NULL,
			role   TEXT NOT NULL,
			kind   TEXT NOT NULL,                 -- 'producer' | 'consumers'
			name   TEXT NOT NULL,
			status TEXT NOT NULL,                 -- 'started' | 'ended'
			detail TEXT NOT NULL
		);
	`, labSchema, produceTable, handlerTable, phaseTable)
	_, err := t.pool.Exec(ctx, createSql)
	return err
}
