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

	"github.com/allegedlyreliable/sqlstreams/.bench/record"
)

// labSchema is the Postgres namespace the checker loads the records into, on
// the same database as sqlstreams's own schema so the checks are plain joins.
const labSchema = "lab"

const (
	progressTable  = "message_progress"
	produceTable   = "produce_record"
	handlerTable   = "handler_record"
	phaseTable     = "run_phase"
	sampleTable    = "observer_sample"
	backlogTable   = "observer_backlog"
	containerTable = "container_sample"
	statementTable = "observer_statement"
)

// the same tables, qualified for the check queries
var (
	messageProgress   = labSchema + "." + progressTable
	produceRecord     = labSchema + "." + produceTable
	handlerRecord     = labSchema + "." + handlerTable
	runPhase          = labSchema + "." + phaseTable
	observerSample    = labSchema + "." + sampleTable
	observerBacklog   = labSchema + "." + backlogTable
	containerSample   = labSchema + "." + containerTable
	observerStatement = labSchema + "." + statementTable
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

var progressLayout = tableLayout{
	kind: record.FileKindProgress, table: progressTable,
	columns: []string{"at", "process", "stream", "group", "attempted", "committed", "rejected", "unknown", "success", "error"},
	decode: func(line []byte) ([]any, error) {
		var row record.ProgressRecord
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		return []any{row.At, row.Process, row.Stream, row.Group, row.Attempted, row.Committed, row.Rejected, row.Unknown, row.Success, row.Error}, nil
	},
}

var produceLayout = tableLayout{
	kind:    record.FileKindProduce,
	table:   produceTable,
	columns: []string{"at", "kind", "stream", "producer", "sequence", "key", "scheduled_at", "message_id", "duplicate", "code", "error"},
	decode: func(line []byte) ([]any, error) {
		var row record.ProduceRecord
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		return []any{row.At, string(row.Kind), row.Stream, row.Producer, row.Sequence, row.Key, row.ScheduledAt, row.MessageId, row.Duplicate, row.Code, row.Error}, nil
	},
}

var handlerLayout = tableLayout{
	kind:    record.FileKindHandler,
	table:   handlerTable,
	columns: []string{"at", "consumer", "stream", "group", "message_id", "key", "attempt", "outcome"},
	decode: func(line []byte) ([]any, error) {
		var row record.HandlerRecord
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		return []any{row.At, row.Consumer, row.Stream, row.Group, row.MessageId, row.Key, row.Attempt, string(row.Outcome)}, nil
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

var sampleLayout = tableLayout{
	kind:    record.FileKindSample,
	table:   sampleTable,
	columns: []string{"at", "wal_records", "wal_fpi", "wal_bytes", "checkpoints", "xact_commit", "deadlocks", "blocks_hit", "blocks_read"},
	decode: func(line []byte) ([]any, error) {
		var row record.SampleRecord
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		return []any{row.At, row.WalRecords, row.WalFpi, row.WalBytes, row.Checkpoints, row.XactCommit, row.Deadlocks, row.BlocksHit, row.BlocksRead}, nil
	},
}

var backlogLayout = tableLayout{
	kind:    record.FileKindBacklog,
	table:   backlogTable,
	columns: []string{"at", "stream", "group", "highest_message", "committed", "highest_allocated"},
	decode: func(line []byte) ([]any, error) {
		var row record.BacklogRecord
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		return []any{row.At, row.Stream, row.Group, row.HighestMessage, row.Committed, row.HighestAllocated}, nil
	},
}

var statementLayout = tableLayout{
	kind:    record.FileKindStatement,
	table:   statementTable,
	columns: []string{"at", "query", "calls", "exec_ms", "rows"},
	decode: func(line []byte) ([]any, error) {
		var row record.StatementRecord
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		return []any{row.At, row.Query, row.Calls, row.ExecMs, row.Rows}, nil
	},
}

// containerLayout has no file kind: stats.sh writes its file on the host,
// outside the record directory, and the checker is handed its path.
var containerLayout = tableLayout{
	table:   containerTable,
	columns: []string{"at", "name", "service", "cpu_percent", "cpus"},
	decode: func(line []byte) ([]any, error) {
		var row record.ContainerRecord
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, err
		}
		return []any{row.At, row.Name, row.Service, row.CpuPercent, row.Cpus}, nil
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

		CREATE TABLE %[1]s.%[8]s (
			at TIMESTAMPTZ NOT NULL, process TEXT NOT NULL, stream TEXT NOT NULL, "group" TEXT NOT NULL,
			attempted BIGINT NOT NULL, committed BIGINT NOT NULL, rejected BIGINT NOT NULL,
			unknown BIGINT NOT NULL, success BIGINT NOT NULL, error BIGINT NOT NULL
		);
		CREATE INDEX %[8]s_identity_at ON %[1]s.%[8]s (process,stream,"group",at);

		CREATE TABLE %[1]s.%[2]s (
			at           TIMESTAMPTZ NOT NULL,
			kind         TEXT NOT NULL,           -- 'attempted' | 'committed' | 'rejected' | 'unknown'
			stream        TEXT NOT NULL,
			producer     TEXT NOT NULL,
			sequence     BIGINT NOT NULL,
			key          TEXT NOT NULL,           -- '<producer>-<sequence>', the idempotency key, unique per stream
			scheduled_at TIMESTAMPTZ NOT NULL,
			message_id   BIGINT NOT NULL,         -- 0 unless committed
			duplicate    BOOLEAN NOT NULL,
			code         TEXT NOT NULL,           -- '' unless rejected
			error        TEXT NOT NULL            -- '' unless rejected or unknown
		);
		CREATE INDEX %[2]s_stream_key ON %[1]s.%[2]s (stream, key);
		CREATE INDEX %[2]s_stream_kind_message_id ON %[1]s.%[2]s (stream, kind, message_id);
		CREATE INDEX %[2]s_kind_scheduled_at ON %[1]s.%[2]s (kind, scheduled_at);

		CREATE TABLE %[1]s.%[3]s (
			at         TIMESTAMPTZ NOT NULL,
			consumer   TEXT NOT NULL,
			stream      TEXT NOT NULL,
			"group"    TEXT NOT NULL,
			message_id BIGINT NOT NULL,
			key        TEXT NOT NULL,
			attempt    INT NOT NULL,
			outcome    TEXT NOT NULL               -- 'success' | 'error'
		);
		CREATE INDEX %[3]s_stream_group_message_id ON %[1]s.%[3]s (stream, "group", message_id);

		CREATE TABLE %[1]s.%[4]s (
			at      TIMESTAMPTZ NOT NULL,
			process TEXT NOT NULL,                -- the writing process's name
			kind    TEXT NOT NULL,                -- 'producer' | 'consumers'
			name    TEXT NOT NULL,
			status  TEXT NOT NULL,                -- 'started' | 'ended'
			detail  TEXT NOT NULL
		);

		CREATE TABLE %[1]s.%[5]s (
			at          TIMESTAMPTZ NOT NULL,
			wal_records BIGINT NOT NULL,          -- cumulative, as the server reports them
			wal_fpi     BIGINT NOT NULL,
			wal_bytes   BIGINT NOT NULL,
			checkpoints BIGINT NOT NULL,
			xact_commit BIGINT NOT NULL,
			deadlocks   BIGINT NOT NULL,
			blocks_hit  BIGINT NOT NULL,
			blocks_read BIGINT NOT NULL
		);
		CREATE INDEX %[5]s_at ON %[1]s.%[5]s (at);

		CREATE TABLE %[1]s.%[6]s (
			at              TIMESTAMPTZ NOT NULL,
			stream           TEXT NOT NULL,
			"group"         TEXT NOT NULL,
			highest_message BIGINT NOT NULL,
            highest_allocated BIGINT NOT NULL,
			committed       BIGINT NOT NULL
		);
		CREATE INDEX %[6]s_at ON %[1]s.%[6]s (at);

		CREATE TABLE %[1]s.%[7]s (
			at          TIMESTAMPTZ NOT NULL,
			name        TEXT NOT NULL,
			service     TEXT NOT NULL,           -- the compose service, 'none' for a container outside the stack
			cpu_percent DOUBLE PRECISION NOT NULL, -- of one core
			cpus        DOUBLE PRECISION NOT NULL  -- the compose cap, 0 when uncapped
		);

		CREATE TABLE %[1]s.%[9]s (
			at      TIMESTAMPTZ NOT NULL,
			query   TEXT NOT NULL,                -- the statement shape, or 'lab' for the observer's own reads
			calls   BIGINT NOT NULL,              -- cumulative, as pg_stat_statements reports them
			exec_ms DOUBLE PRECISION NOT NULL,
			rows    BIGINT NOT NULL
		);
		CREATE INDEX %[9]s_query_at ON %[1]s.%[9]s (query, at);
	`, labSchema, produceTable, handlerTable, phaseTable, sampleTable, backlogTable, containerTable, progressTable, statementTable)
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

func (d *CheckerDatastore) LoadSample(ctx context.Context, dir string) (int64, error) {
	return d.load(ctx, dir, sampleLayout)
}

func (d *CheckerDatastore) LoadBacklog(ctx context.Context, dir string) (int64, error) {
	return d.load(ctx, dir, backlogLayout)
}

func (d *CheckerDatastore) LoadStatement(ctx context.Context, dir string) (int64, error) {
	return d.load(ctx, dir, statementLayout)
}

// LoadContainer COPYs the one file stats.sh wrote at path.
func (d *CheckerDatastore) LoadContainer(ctx context.Context, path string) (int64, error) {
	rows, err := d.loadFile(ctx, path, containerLayout)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return rows, nil
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
	// COPY leaves fresh tables without planner statistics until autoanalyze.
	analyzeSql := fmt.Sprintf(`
		-- lab: datastore.load
		ANALYZE %s;
	`, pgx.Identifier{labSchema, layout.table}.Sanitize())
	_, err = d.pool.Exec(ctx, analyzeSql)
	return loaded, err
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

func (d *CheckerDatastore) LoadProgress(ctx context.Context, dir string) (int64, error) {
	return d.load(ctx, dir, progressLayout)
}

// ReloadProgress replaces the loaded snapshots with the files as they are
// now: the consumers keep writing while the checker drains them.
func (d *CheckerDatastore) ReloadProgress(ctx context.Context, dir string) (int64, error) {
	truncateSql := fmt.Sprintf(`
		-- lab: datastore.ReloadProgress
		TRUNCATE %s;
	`, messageProgress)
	if _, err := d.pool.Exec(ctx, truncateSql); err != nil {
		return 0, err
	}
	return d.load(ctx, dir, progressLayout)
}
