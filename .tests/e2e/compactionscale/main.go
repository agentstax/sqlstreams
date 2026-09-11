package main

// Log compaction SCALE e2e test: how bad does "prove a negative" actually get as
// a stream's history grows? compactionwidth (11 partitions) showed the
// SHAPE -- a full scan, no early termination. This e2e test pushes the same
// question to the case that actually worries someone running this in
// production: a backlog consumer replaying a never-superseded key from near
// the start of a long-lived, high-volume stream, where "current tail" keeps
// getting further away as the stream ages.
//
// One row (id=1, message_key="stale") is never superseded. The stream's
// history is grown in checkpoints -- more partitions, more filler rows
// behind it -- and at each checkpoint the SAME row's "is this the latest"
// check is EXPLAIN ANALYZEd fresh, so partitions-touched and wall-clock
// execution time are tracked as a genuine growth curve, not one snapshot.
//
// Partitions/rows are seeded via bulk DDL + a single set-based INSERT per
// checkpoint (not one Produce() round trip per row) -- this e2e test cares about
// query cost at scale, not seeding realism, and thousands of individual
// round trips would make it impractically slow.

import (
	"context"
	"fmt"
	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
)

const partitionSize = int64(10)

// partition counts to measure at -- each step extends the SAME stream rather
// than reseeding from scratch, so total seeding work is proportional to the
// final size, not the sum of every checkpoint.
//
// capped at 1000: stream.Destroy's cleanup DROPs the whole partitioned table
// in one transaction, which needs one lock per partition -- past a few
// thousand that exceeds Postgres's default max_locks_per_transaction and
// the e2e test's own teardown fails with "out of shared memory." That's a real
// limit worth knowing about, but it's an operational ceiling on partition
// COUNT itself, a different question from what this e2e test measures.
var checkpoints = []int64{10, 50, 200, 500, 1000}

type result struct {
	partitions int64
	rows       int64
	touched    int
	execMs     float64
}

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() (err error) {
	defer common.Recover(&err)
	ctx := context.Background()

	pool, err := common.NewPool(ctx, nil)
	common.Must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	streamName := fmt.Sprintf("phase8c.compactionscale.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: partitionSize})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	step("insert the never-superseded row -- id=1, message_key=\"stale\"")
	insertStaleRow(ctx, ds, tp.Id)

	step("grow the stream's history in checkpoints, measuring the SAME row's negative-proof cost fresh each time")
	fmt.Printf("  %-12s %-10s %-10s %-10s\n", "partitions", "rows", "touched", "exec_ms")

	var createdPartitions int64 = 1 // partition 0 already exists from Register
	var totalRows int64 = 1
	results := make([]result, 0, len(checkpoints))
	var firstPlan string

	for i, target := range checkpoints {
		createPartitions(ctx, ds, tp.Id, createdPartitions, target)
		createdPartitions = target

		// target*partitionSize - 1 -- the highest id [target] partitions cover: partition
		// k spans [k*size, (k+1)*size), 0-based, but BIGSERIAL ids start at 1, not 0.
		targetRows := target*partitionSize - 1
		bulkInsertFiller(ctx, ds, tp.Id, targetRows-totalRows)
		totalRows = targetRows

		touched, execMs, plan := explainStaleNegative(ctx, ds, tp.Id)
		results = append(results, result{partitions: target, rows: totalRows, touched: touched, execMs: execMs})
		fmt.Printf("  %-12d %-10d %-10d %-10.3f\n", target, totalRows, touched, execMs)
		if i == 0 {
			firstPlan = plan
		}
	}

	step("full plan at the smallest checkpoint (readable at this size; the shape is identical at every larger one)")
	fmt.Print(firstPlan)

	step("what the growth curve says")
	first, last := results[0], results[len(results)-1]
	assertTrue(fmt.Sprintf("touched partitions grew with history size (%d -> %d)", first.touched, last.touched),
		last.touched > first.touched)
	assertTrue(fmt.Sprintf("execution time grew with history size (%.3fms -> %.3fms)", first.execMs, last.execMs),
		last.execMs > first.execMs)
	fmt.Printf("  -> %d -> %d partitions of history (%.0fx) drove touched partitions %d -> %d and cost %.3fms -> %.3fms\n",
		first.partitions, last.partitions, float64(last.partitions)/float64(first.partitions),
		first.touched, last.touched, first.execMs, last.execMs)
	fmt.Println("  -> nothing amortizes this: every checkpoint re-measures the IDENTICAL row, older with")
	fmt.Println("     each step only because MORE history piled up behind it, never resolved cheaper")

	fmt.Println("\n✅ COMPACTION SCALE E2E TEST — numbers gathered; decision records [0261]/[0263] (.docs/decisions/) hold what was decided on them")
	return nil
}

// ---- helpers ----

func insertStaleRow(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) {
	sql := fmt.Sprintf(`INSERT INTO %s.%s (payload, schema_version, message_key, compaction_rank) VALUES ('{}'::jsonb, 1, 'stale', 0);`, ds.Schema, stream.MessageLogTable(streamId))
	_, err := ds.Pool.Exec(ctx, sql)
	common.Must(err)
}

// createPartitions issues every CREATE TABLE ... PARTITION OF statement for
// [from, to) as ONE multi-statement Exec -- a network round trip per
// partition would dominate the e2e test's own runtime at these checkpoint sizes.
func createPartitions(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId, from, to int64) {
	if to <= from {
		return
	}
	logName := stream.MessageLogTable(streamId)
	logTable := fmt.Sprintf("%s.%s", ds.Schema, logName)
	var sql strings.Builder
	for n := from; n < to; n++ {
		fmt.Fprintf(&sql, "CREATE TABLE IF NOT EXISTS %s_%d PARTITION OF %s FOR VALUES FROM (%d) TO (%d);\n",
			logTable, n, logTable, n*partitionSize, (n+1)*partitionSize)
	}
	_, err := ds.Pool.Exec(ctx, sql.String())
	common.Must(err)
}

// bulkInsertFiller adds `count` unkeyed rows in one set-based INSERT --
// unkeyed traffic never touches the compaction subplan (compaction
// already proved that), so it's free filler for growing the stream's row
// count/tail position without affecting what's being measured.
func bulkInsertFiller(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId, count int64) {
	if count <= 0 {
		return
	}
	sql := fmt.Sprintf(`
		INSERT INTO %s.%s (payload, schema_version, message_key)
		SELECT '{}'::jsonb, 1, NULL FROM generate_series(1, $1);
	`, ds.Schema, stream.MessageLogTable(streamId))
	_, err := ds.Pool.Exec(ctx, sql, count)
	common.Must(err)
}

// explainStaleNegative EXPLAIN ANALYZEs whether id=1 ("stale") is still the
// latest for its key, counting only partitions the Append node ACTUALLY
// EXECUTED against (see compactionwidth for why mentions alone don't
// mean touched), plus the plan's own reported wall-clock Execution Time.
func explainStaleNegative(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) (int, float64, string) {
	logName := stream.MessageLogTable(streamId)
	logTable := fmt.Sprintf("%s.%s", ds.Schema, logName)
	sql := fmt.Sprintf(`
		EXPLAIN (ANALYZE, COSTS OFF) SELECT 1 FROM %s m
		WHERE m.id = 1
			AND NOT EXISTS (
				SELECT 1 FROM %s newer
				WHERE newer.message_key = m.message_key
					AND newer.id > m.id
			);
	`, logTable, logTable)

	rows, err := ds.Pool.Query(ctx, sql)
	common.Must(err)
	defer rows.Close()

	partitionRe := regexp.MustCompile(regexp.QuoteMeta(logName) + `_\d+`)
	execRe := regexp.MustCompile(`Execution Time: ([\d.]+) ms`)
	executed := map[string]bool{}
	var execMs float64
	var plan strings.Builder
	for rows.Next() {
		var line string
		common.Must(rows.Scan(&line))
		plan.WriteString(line)
		plan.WriteString("\n")

		if m := execRe.FindStringSubmatch(line); m != nil {
			execMs, _ = strconv.ParseFloat(m[1], 64)
		}
		matches := partitionRe.FindAllString(line, -1)
		if len(matches) == 0 || strings.Contains(line, "never executed") {
			continue
		}
		for _, p := range matches {
			executed[p] = true
		}
	}
	common.Must(rows.Err())
	return len(executed), execMs, plan.String()
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
func assertTrue(label string, cond bool) {
	if !cond {
		common.Die(fmt.Sprintf("%s: got false, want true", label))
	}
	fmt.Printf("  ✓ %s\n", label)
}
