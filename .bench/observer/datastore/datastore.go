package datastore

// datastore is every query the observer runs, all reads: the server's own
// statistics views and one consumer group's position in sqlstreams's tables.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/allegedlyreliable/sqlstreams/.bench/record"
	sqlstreamsdatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// sqlstreamsSchema is where the roles' client put sqlstreams's tables: they run with
// a nil ClientConfig, so the default.
const sqlstreamsSchema = sqlstreamsdatastore.DefaultSchema

// labMarker sits inside every read the observer runs each second, past the
// first token so pg_stat_statements keeps it, and ReadStatements sums those
// reads under "lab" instead of counting them against the deployment.
const labMarker = "/* lab */"

// statementTextLimit cuts a statement shape: enough to recognize the verb
// and its table, short enough that a DDL batch does not swell the records.
const statementTextLimit = 200

type ObserverDatastore struct {
	pool *pgxpool.Pool
}

func NewObserverDatastore(pool *pgxpool.Pool) (*ObserverDatastore, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	return &ObserverDatastore{pool: pool}, nil
}

// Target is the stream and consumer group whose position is sampled,
// resolved from the catalog by the names the scenario declares.
type Target struct {
	StreamId int64
	GroupId  int64
}

// ReadSample reads the server's cumulative counters: WAL, checkpoints, and
// the current database's transactions, deadlocks, and buffer traffic. At is
// left for the caller to set.
func (d *ObserverDatastore) ReadSample(ctx context.Context) (record.SampleRecord, error) {
	sampleSql := fmt.Sprintf(`
		-- lab: datastore.ReadSample
		SELECT %[1]s
			w.wal_records,
			w.wal_fpi,
			w.wal_bytes,
			c.num_done,
			d.xact_commit,
			d.deadlocks,
			d.blks_hit,
			d.blks_read
		FROM pg_stat_wal w, pg_stat_checkpointer c, pg_stat_database d
		WHERE d.datname = current_database();
	`, labMarker)
	var sample record.SampleRecord
	err := d.pool.QueryRow(ctx, sampleSql).Scan(
		&sample.WalRecords, &sample.WalFpi, &sample.WalBytes, &sample.Checkpoints,
		&sample.XactCommit, &sample.Deadlocks, &sample.BlocksHit, &sample.BlocksRead)
	return sample, err
}

// CreateStatementsExtension makes pg_stat_statements readable on the lab
// database; the server must have loaded the module at start.
func (d *ObserverDatastore) CreateStatementsExtension(ctx context.Context) error {
	createSql := `
		-- lab: datastore.CreateStatementsExtension
		CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
	`
	_, err := d.pool.Exec(ctx, createSql)
	return err
}

// ReadStatements reads pg_stat_statements summed per statement shape. The
// stored text starts at the statement's first token, so the library's
// leading owner comment is gone; the shape is the text with whitespace
// collapsed, per-stream table suffixes folded to _N, and cut at
// statementTextLimit. The observer's own reads carry the labMarker and sum
// under "lab". At is left for the caller to set.
func (d *ObserverDatastore) ReadStatements(ctx context.Context) ([]record.StatementRecord, error) {
	statementsSql := fmt.Sprintf(`
		-- lab: datastore.ReadStatements
		SELECT %[1]s
			CASE
				WHEN position('%[2]s' IN query) > 0 THEN 'lab'
				ELSE left(regexp_replace(regexp_replace(query, '\s+', ' ', 'g'), '_[0-9]+', '_N', 'g'), %[3]d)
			END AS shape,
			sum(calls),
			sum(total_exec_time),
			sum(rows)
		FROM pg_stat_statements
		GROUP BY 1;
	`, labMarker, labMarker, statementTextLimit)
	rows, err := d.pool.Query(ctx, statementsSql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	statements := []record.StatementRecord{}
	for rows.Next() {
		var statement record.StatementRecord
		if err := rows.Scan(&statement.Query, &statement.Calls, &statement.ExecMs, &statement.Rows); err != nil {
			return nil, err
		}
		statements = append(statements, statement)
	}
	return statements, rows.Err()
}

// ResolveTarget is comma-ok: false until the consumer role has registered
// the stream and group.
func (d *ObserverDatastore) ResolveTarget(ctx context.Context, streamName string, groupName string) (Target, bool, error) {
	resolveSql := fmt.Sprintf(`
		-- lab: datastore.ResolveTarget
		SELECT %[2]s t.id, g.id
		FROM %[1]s.stream_config t
		JOIN %[1]s.consumer_group_config g ON g.stream_id = t.id AND g.name = $2
		WHERE t.name = $1;
	`, sqlstreamsSchema, labMarker)
	var resolved Target
	err := d.pool.QueryRow(ctx, resolveSql, streamName, groupName).Scan(&resolved.StreamId, &resolved.GroupId)
	if errors.Is(err, pgx.ErrNoRows) {
		return Target{}, false, nil
	}
	if err != nil {
		return Target{}, false, err
	}
	return resolved, true, nil
}

// ReadBacklog uses allocated ids as an upper bound, including uncommitted ids and gaps.
// Reading the sequence avoids taking locks on message partitions during cleanup.
func (d *ObserverDatastore) ReadBacklog(ctx context.Context, target Target) (record.BacklogRecord, error) {
	backlogSql := fmt.Sprintf(`
		-- lab: datastore.ReadBacklog
		SELECT %[3]s
			(SELECT CASE WHEN is_called THEN last_value ELSE 0 END FROM %[1]s),
			(SELECT COALESCE(max(committed), 0) FROM %[2]s WHERE consumer_group_id = $1);
	`, sqlstreamsSchema+"."+stream.MessageLogIdSequence(target.StreamId), sqlstreamsSchema+"."+stream.ConsumerGroupCursorTable(target.StreamId), labMarker)
	var backlog record.BacklogRecord
	err := d.pool.QueryRow(ctx, backlogSql, target.GroupId).Scan(&backlog.HighestAllocated, &backlog.Committed)
	return backlog, err
}
