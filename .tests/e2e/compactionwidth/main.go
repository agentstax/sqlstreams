package main

// Log compaction width/planner e2e test: measures the read-cost tradeoff the
// unbounded compaction predicate creates (decision records [0261]/[0263] in
// .docs/decisions/ -- this e2e test is what turns the tradeoff into a number).
//
// Proving a row IS the latest for its key (NOT EXISTS a newer one) has no
// early termination -- it costs one partition scan per partition from that
// row's own partition through the stream's CURRENT last one. Proving it
// ISN'T (a newer row exists somewhere) can stop as soon as a match is
// found, wherever that happens to be.
//
// Registers two streams seeded with the IDENTICAL 40-message workload,
// differing only in PartitionSize (narrow vs wide, an order of magnitude
// apart), so the same two EXPLAIN checks can be compared side by side:
//   - the first message ("stale") is never superseded -- the "prove a
//     negative" case.
//   - the last two ("fresh" v1/v2) are two versions published back to back
//     -- the "find a match" case, with the match one partition away at most.

import (
	"context"
	"fmt"
	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
)

const (
	narrowPartitionSize = int64(4)
	// wide enough that 40 seeded rows stay below the 80% create-ahead trigger
	// (id 80) -- at 50, id 40 IS the trigger and an empty partition 1 appears
	widePartitionSize = int64(100)
)

type Record struct {
	Key string `json:"key"`
}

func (Record) SchemaVersion() int { return 1 }

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

	narrowName := fmt.Sprintf("phase8c.compactionwidth.narrow.%d", time.Now().UnixNano())
	narrow, err := client.Stream[sqlstreams.RawPayload](narrowName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: narrowPartitionSize})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](narrowName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	wideName := fmt.Sprintf("phase8c.compactionwidth.wide.%d", time.Now().UnixNano())
	wide, err := client.Stream[sqlstreams.RawPayload](wideName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: widePartitionSize})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](wideName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	step("seed both streams with the identical 40-message workload")
	narrowProducerInstance, err := client.Stream[Record](narrow.Name).Producer().Register(ctx, nil)
	common.Must(err)
	wideProducerInstance, err := client.Stream[Record](wide.Name).Producer().Register(ctx, nil)
	common.Must(err)
	seed(ctx, narrowProducerInstance)
	seed(ctx, wideProducerInstance)

	narrowPartitions := countPartitions(ctx, ds, narrow.Id)
	widePartitions := countPartitions(ctx, ds, wide.Id)
	fmt.Printf("  narrow: PartitionSize=%d -> %d partitions\n", narrowPartitionSize, narrowPartitions)
	fmt.Printf("  wide:   PartitionSize=%d -> %d partition(s)\n", widePartitionSize, widePartitions)

	// each boundary heal burns an id on a rolled-back insert, so the narrow
	// stream's ids drift -- read the seeded rows' real ids back instead of
	// hard-coding them
	narrowStale := keyId(ctx, ds, narrow.Id, "stale", "MIN")
	narrowFreshV1 := keyId(ctx, ds, narrow.Id, "fresh", "MIN")
	wideStale := keyId(ctx, ds, wide.Id, "stale", "MIN")
	wideFreshV1 := keyId(ctx, ds, wide.Id, "fresh", "MIN")

	step("narrow stream: EXPLAIN the compaction check for the negative and match cases")
	negNarrow, negNarrowPlan := explainCompactionTouches(ctx, ds, narrow.Id, narrowStale, "prove a negative (\"stale\")")
	posNarrow, posNarrowPlan := explainCompactionTouches(ctx, ds, narrow.Id, narrowFreshV1, "find a match (\"fresh\" v1)")
	fmt.Println("\n  --- narrow / negative case plan ---")
	fmt.Print(negNarrowPlan)
	fmt.Println("  --- narrow / match case plan ---")
	fmt.Print(posNarrowPlan)

	step("wide stream: same two checks")
	negWide, _ := explainCompactionTouches(ctx, ds, wide.Id, wideStale, "prove a negative (\"stale\")")
	posWide, _ := explainCompactionTouches(ctx, ds, wide.Id, wideFreshV1, "find a match (\"fresh\" v1)")

	step("what the numbers say")
	assertTrue("narrow: proving a negative touches more partitions than finding a match",
		negNarrow > posNarrow)
	assertTrue("narrow: proving a negative touches nearly every partition -- no early termination",
		int64(negNarrow) >= narrowPartitions-1)
	// 40 messages fit one wide partition, so both cases collapse to a single
	// scan -- the width tradeoff only exists once data spans partitions
	assertTrue("wide: both cases stay inside the one partition all 40 rows share",
		negWide == 1 && posWide == 1)

	fmt.Println("\n✅ COMPACTION WIDTH E2E TEST — numbers gathered; decision records [0261]/[0263]")
	fmt.Println("   (.docs/decisions/) hold what they mean and what was decided on them.")
	return nil
}

// ---- helpers ----

// seed publishes the SAME 40-message shape regardless of stream: first a key
// that's never superseded, then 37 unique fillers (each its own key, so none
// of them ever match another row's compaction subplan), then two versions of
// one key published back to back. Partition boundaries self-heal on the
// produce path.
func seed(ctx context.Context, wp *sqlstreams.ProducerInstance[Record]) {
	publish(ctx, wp, "stale") // never superseded
	for i := range 37 {
		publish(ctx, wp, fmt.Sprintf("filler:%d", i)) // each a distinct key
	}
	publish(ctx, wp, "fresh") // v1
	publish(ctx, wp, "fresh") // v2 -- immediately supersedes v1
}

func publish(ctx context.Context, wp *sqlstreams.ProducerInstance[Record], key string) {
	_, err := wp.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*Record, error) {
		return &Record{Key: key}, nil
	}, &sqlstreams.ProduceOptions{MessageKey: key, Compaction: &sqlstreams.CompactionOptions{Enable: true}})
	common.Must(err)
}

// keyId reads back one seeded row's real id -- aggregate is MIN or MAX,
// picking between the two versions of a twice-published key.
func keyId(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, messageKey string, aggregate string) int64 {
	return scalar(ctx, ds, fmt.Sprintf(`
		SELECT %s(id) FROM %s.%s WHERE message_key = $1;
	`, aggregate, ds.Schema, stream.MessageLogTable(streamId)), messageKey)
}

func countPartitions(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) int64 {
	return scalar(ctx, ds, fmt.Sprintf(`
		SELECT count(*) FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = '%s.%s'::regclass;
	`, ds.Schema, stream.MessageLogTable(streamId)))
}

// explainCompactionTouches EXPLAIN ANALYZEs just the compaction predicate
// (isolated from the bindings clause, which is a separate concern) for one
// row, and counts partitions the Append node ACTUALLY EXECUTED against.
//
// Every partition the "newer" subplan could statically apply to is always
// LISTED in the plan (Append always enumerates every child), so counting
// mentions alone can't tell scanned from skipped -- Postgres tags a child
// "(never executed)" when the anti-join's early termination (or runtime
// partition pruning) meant it was never actually opened. Only lines WITHOUT
// that tag count as a real touch.
func explainCompactionTouches(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId, id int64, label string) (int, string) {
	logName := stream.MessageLogTable(streamId)
	logTable := fmt.Sprintf("%s.%s", ds.Schema, logName)
	sql := fmt.Sprintf(`
		EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF) SELECT 1 FROM %s m
		WHERE m.id = $1
			AND NOT EXISTS (
				SELECT 1 FROM %s newer
				WHERE newer.message_key = m.message_key
					AND newer.id > m.id
			);
	`, logTable, logTable)

	rows, err := ds.Pool.Query(ctx, sql, id)
	common.Must(err)
	defer rows.Close()

	partitionRe := regexp.MustCompile(regexp.QuoteMeta(logName) + `_\d+`)
	executed := map[string]bool{}
	var plan strings.Builder
	for rows.Next() {
		var line string
		common.Must(rows.Scan(&line))
		plan.WriteString(line)
		plan.WriteString("\n")
		matches := partitionRe.FindAllString(line, -1)
		if len(matches) == 0 {
			continue
		}
		if strings.Contains(line, "never executed") {
			continue // listed in the plan, but the Append never actually opened it
		}
		for _, m := range matches {
			executed[m] = true
		}
	}
	common.Must(rows.Err())

	names := make([]string, 0, len(executed))
	for n := range executed {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Printf("  %s: ACTUALLY EXECUTED against %d partition(s): %v\n", label, len(names), names)
	return len(names), plan.String()
}

func scalar(ctx context.Context, ds *iDatastore.PostgresDatastore, q string, args ...any) int64 {
	var v int64
	common.Must(ds.Pool.QueryRow(ctx, q, args...).Scan(&v))
	return v
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
func assertTrue(label string, cond bool) {
	if !cond {
		common.Die(fmt.Sprintf("%s: got false, want true", label))
	}
	fmt.Printf("  ✓ %s\n", label)
}
