package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
)

// tableLayout is how one file kind lands in its table: the columns in the
// order decode returns them.
type tableLayout struct {
	table   string
	columns []string
	decode  func(line []byte) ([]any, error)
}

var layouts = map[FileKind]tableLayout{
	FileProduce: {
		table:   ProduceTable,
		columns: []string{"at", "kind", "producer", "seq", "key", "scheduled_at", "message_id", "duplicate", "code", "error"},
		decode: func(line []byte) ([]any, error) {
			var fact ProduceFact
			if err := json.Unmarshal(line, &fact); err != nil {
				return nil, err
			}
			return []any{fact.At, string(fact.Kind), fact.Producer, fact.Seq, fact.Key, fact.ScheduledAt, fact.MessageId, fact.Duplicate, fact.Code, fact.Error}, nil
		},
	},
	FileHandler: {
		table:   HandlerTable,
		columns: []string{"at", "consumer", "group", "message_id", "key", "attempt", "outcome"},
		decode: func(line []byte) ([]any, error) {
			var fact HandlerFact
			if err := json.Unmarshal(line, &fact); err != nil {
				return nil, err
			}
			return []any{fact.At, fact.Consumer, fact.Group, fact.MessageId, fact.Key, fact.Attempt, string(fact.Outcome)}, nil
		},
	},
	FilePhase: {
		table:   PhaseTable,
		columns: []string{"at", "role", "kind", "name", "status", "detail"},
		decode: func(line []byte) ([]any, error) {
			var fact PhaseFact
			if err := json.Unmarshal(line, &fact); err != nil {
				return nil, err
			}
			return []any{fact.At, fact.Role, string(fact.Kind), fact.Name, string(fact.Status), fact.Detail}, nil
		},
	},
}

// Load COPYs every <name>.<kind>.jsonl of one kind under dir into the
// kind's table, one streamed COPY per file, and returns the rows loaded. The
// checker loads the produce files before it drains and the handler files
// after, so each table is read once it is complete. A line that does not
// decode is an error.
func (t *Tables) Load(ctx context.Context, dir string, kind FileKind) (int64, error) {
	layout, ok := layouts[kind]
	if !ok {
		return 0, fmt.Errorf("unrecognized ledger file kind: %q", string(kind))
	}
	if _, err := os.Stat(dir); err != nil {
		return 0, err
	}
	paths, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("*.%s.jsonl", kind)))
	if err != nil {
		return 0, err
	}

	var loaded int64
	for _, path := range paths {
		rows, err := t.loadFile(ctx, path, layout)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		loaded += rows
	}
	return loaded, nil
}

func (t *Tables) loadFile(ctx context.Context, path string, layout tableLayout) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	source := newLineSource(file, layout.decode)
	rows, err := t.pool.CopyFrom(ctx, pgx.Identifier{Schema, layout.table}, layout.columns, source)
	if err != nil {
		return 0, err
	}
	return rows, nil
}
