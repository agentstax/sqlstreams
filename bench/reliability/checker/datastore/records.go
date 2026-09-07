package datastore

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"

	"github.com/agentstax/vulkan/bench/reliability/record"
)

// labSchema is the Postgres namespace the checker loads the records into, on
// the same database as vulkan's own schema so the checks are plain joins.
const labSchema = "lab"

const (
	produceTable = "produce_record"
	handlerTable = "handler_record"
	phaseTable   = "run_phase"
)

// the same tables, qualified for the check queries
var (
	produceRecord = labSchema + "." + produceTable
	handlerRecord = labSchema + "." + handlerTable
	runPhase      = labSchema + "." + phaseTable
)

// lineByteLimit bounds one record line; the longest field is an error text.
const lineByteLimit = 1 << 20

// tableLayout is how one record file kind lands in its table: the columns
// in the order decode returns them.
type tableLayout struct {
	kind    record.FileKind
	table   string
	columns []string
	decode  func(line []byte) ([]any, error)
}

var produceLayout = tableLayout{
	kind:    record.FileKindProduce,
	table:   produceTable,
	columns: []string{"at", "kind", "producer", "sequence", "key", "scheduled_at", "message_id", "duplicate", "code", "error"},
	decode: func(line []byte) ([]any, error) {
		var row record.ProduceRecord
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		return []any{row.At, string(row.Kind), row.Producer, row.Sequence, row.Key, row.ScheduledAt, row.MessageId, row.Duplicate, row.Code, row.Error}, nil
	},
}

var handlerLayout = tableLayout{
	kind:    record.FileKindHandler,
	table:   handlerTable,
	columns: []string{"at", "consumer", "group", "message_id", "key", "attempt", "outcome"},
	decode: func(line []byte) ([]any, error) {
		var row record.HandlerRecord
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		return []any{row.At, row.Consumer, row.Group, row.MessageId, row.Key, row.Attempt, string(row.Outcome)}, nil
	},
}

var phaseLayout = tableLayout{
	kind:    record.FileKindPhase,
	table:   phaseTable,
	columns: []string{"at", "process", "kind", "name", "status", "detail"},
	decode: func(line []byte) ([]any, error) {
		var row record.PhaseRecord
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		return []any{row.At, row.Process, string(row.Kind), row.Name, string(row.Status), row.Detail}, nil
	},
}

// lineSource feeds COPY one decoded JSON line at a time, so a file is never
// held in memory whole.
type lineSource struct {
	scanner *bufio.Scanner
	decode  func(line []byte) ([]any, error)
	line    int
	row     []any
	err     error
}

var _ pgx.CopyFromSource = (*lineSource)(nil)

func newLineSource(reader io.Reader, decode func(line []byte) ([]any, error)) *lineSource {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(nil, lineByteLimit)
	return &lineSource{scanner: scanner, decode: decode}
}

func (s *lineSource) Next() bool {
	if !s.scanner.Scan() {
		s.err = s.scanner.Err()
		return false
	}
	s.line++

	row, err := s.decode(s.scanner.Bytes())
	if err != nil {
		s.err = fmt.Errorf("line %d: %w", s.line, err)
		return false
	}
	s.row = row
	return true
}

func (s *lineSource) Values() ([]any, error) {
	return s.row, nil
}

func (s *lineSource) Err() error {
	return s.err
}

// CreateTables drops and recreates labSchema with one table per record file
// kind, the JSON-lines field names as columns, so a checker run reads only
// the files it loaded.
func (d *CheckerDatastore) CreateTables(ctx context.Context) error {
	createSql := fmt.Sprintf(`
		-- lab: datastore.CreateTables
		DROP SCHEMA IF EXISTS %[1]s CASCADE;
		CREATE SCHEMA %[1]s;

		CREATE TABLE %[1]s.%[2]s (
			at           TIMESTAMPTZ NOT NULL,
			kind         TEXT NOT NULL,           -- 'attempted' | 'committed' | 'rejected' | 'unknown'
			producer     TEXT NOT NULL,
			sequence          BIGINT NOT NULL,
			key          TEXT NOT NULL,           -- '<producer>-<sequence>', the idempotency key
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
			at      TIMESTAMPTZ NOT NULL,
			process TEXT NOT NULL,                -- the writing process's name
			kind    TEXT NOT NULL,                -- 'producer' | 'consumers'
			name    TEXT NOT NULL,
			status  TEXT NOT NULL,                -- 'started' | 'ended'
			detail  TEXT NOT NULL
		);
	`, labSchema, produceTable, handlerTable, phaseTable)
	_, err := d.pool.Exec(ctx, createSql)
	return err
}

// LoadProduce, LoadHandler, and LoadPhase COPY every <name>.<kind>.jsonl of
// their kind under dir into the kind's table, one streamed COPY per file,
// and return the rows loaded. A line that does not decode is an error.
func (d *CheckerDatastore) LoadProduce(ctx context.Context, dir string) (int64, error) {
	return d.load(ctx, dir, produceLayout)
}

func (d *CheckerDatastore) LoadHandler(ctx context.Context, dir string) (int64, error) {
	return d.load(ctx, dir, handlerLayout)
}

func (d *CheckerDatastore) LoadPhase(ctx context.Context, dir string) (int64, error) {
	return d.load(ctx, dir, phaseLayout)
}

func (d *CheckerDatastore) load(ctx context.Context, dir string, layout tableLayout) (int64, error) {
	paths, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("*.%s.jsonl", layout.kind)))
	if err != nil {
		return 0, err
	}

	var loaded int64
	for _, path := range paths {
		rows, err := d.loadFile(ctx, path, layout)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		loaded += rows
	}
	return loaded, nil
}

func (d *CheckerDatastore) loadFile(ctx context.Context, path string, layout tableLayout) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	source := newLineSource(file, layout.decode)
	return d.pool.CopyFrom(ctx, pgx.Identifier{labSchema, layout.table}, layout.columns, source)
}
