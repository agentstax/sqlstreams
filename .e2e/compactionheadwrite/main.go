package main

// compaction_head write-cost e2e test: quantifies the tradeoff this phase's design
// made deliberately but never measured -- read-path scans got O(1), at the
// cost of a second write (an UPSERT into compaction_head, same transaction) on
// every keyed publish. Three scenarios:
//
//   - Fixed cost: sequential, uncontended publishes -- unkeyed vs. a fresh
//     key each time (pure INSERT into compaction_head) vs. the SAME key every
//     time (the ON CONFLICT DO UPDATE branch). Isolates the extra
//     statement's own cost from any lock contention.
//   - Hot-key contention: G goroutines concurrently publish -- each to its
//     OWN distinct key (parallel compaction_head rows, no contention) vs. all G
//     to the SAME single key (serialized on that one row, the "known
//     tradeoff" flagged back in the design but never measured under load).
//   - Dead-tuple growth: the hot-key scenario repeatedly UPDATEs ONE row --
//     n_dead_tup/n_tup_upd on compaction_head before and after the burst shows
//     what that does to table bloat, separate from the latency question.
//
// Registers its own streams (destroyed on exit), self-seeded, self-verifying.

import (
	"context"
	"fmt"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"os"
	"sync"
	"time"

	"github.com/agentstax/sqlstreams/e2e/common"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/jackc/pgx/v5/pgxpool"
)

const largePartitionSize = int64(1000000) // never rolls -- partition churn isn't what's being measured

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

// testFailure is what die panics with; run recovers it into its error so
// main's deferred cleanup runs on a failed assertion.
type testFailure struct {
	message string
}

func (f testFailure) Error() string {
	return f.message
}

func run() (err error) {
	defer func() {
		switch recovered := recover().(type) {
		case nil:
		case testFailure:
			err = recovered
		default:
			panic(recovered)
		}
	}()
	ctx := context.Background()

	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", &sqlstreams.PostgresConnectionConfig{
		MaxConns: 60, // headroom above the hot-key scenario's 50 concurrent goroutines
	})
	must(err)
	defer pool.Close()

	fixedCostScenario(ctx, pool)
	hotKeyContentionScenario(ctx, pool)

	fmt.Println("\n✅ LATEST KEYS WRITE-COST E2E TEST — numbers gathered; decision record [0262]")
	fmt.Println("   (.docs/decisions/) holds the write-per-keyed-publish tradeoff they measure.")
	return nil
}

// fixedCostScenario: N sequential, single-threaded publishes per case --
// zero contention, so the only thing the timing difference can reflect is
// the extra statement itself (and INSERT vs. UPDATE within it).
func fixedCostScenario(ctx context.Context, pool *pgxpool.Pool) {
	step("fixed cost: sequential publishes, no contention -- unkeyed vs. fresh-key INSERT vs. same-key UPDATE")

	const n = 500
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	must(err)

	streamName := fmt.Sprintf("phase8c.compactionheadwrite.fixed.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: largePartitionSize})
	must(err)
	defer func() {
		must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	must(err)

	unkeyedMs := timeSequential(ctx, wpInstance, n, func(i int) string { return "" })
	freshKeyMs := timeSequential(ctx, wpInstance, n, func(i int) string { return fmt.Sprintf("fresh-%d", i) })
	sameKeyMs := timeSequential(ctx, wpInstance, n, func(i int) string { return "same-key" })

	fmt.Printf("  %-28s %10.3fms total  %8.4fms/op\n", "unkeyed (baseline)", unkeyedMs, unkeyedMs/n)
	fmt.Printf("  %-28s %10.3fms total  %8.4fms/op  (+%.1f%% vs. baseline)\n", "fresh key (compaction_head INSERT)", freshKeyMs, freshKeyMs/n, pctOver(freshKeyMs, unkeyedMs))
	fmt.Printf("  %-28s %10.3fms total  %8.4fms/op  (+%.1f%% vs. baseline)\n", "same key (compaction_head UPDATE)", sameKeyMs, sameKeyMs/n, pctOver(sameKeyMs, unkeyedMs))
}

// hotKeyContentionScenario: the design's own flagged-but-unmeasured tradeoff
// -- concurrent publishes to the SAME key now serialize on that key's
// compaction_head row, where plain message_log appends never contended before.
func hotKeyContentionScenario(ctx context.Context, pool *pgxpool.Pool) {
	step("hot-key contention: G concurrent publishers, each to its OWN key vs. all G to ONE key")

	const goroutines = 50
	const perGoroutine = 20

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	must(err)

	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	must(err)

	manyKeysMs, manyKeysStream := timeConcurrent(ctx, pool, "manykeys", goroutines, perGoroutine, func(g, i int) string {
		return fmt.Sprintf("key-%d", g) // each goroutine owns a distinct key -- no cross-goroutine contention
	})
	defer func() {
		must(client.Stream[sqlstreams.RawPayload](manyKeysStream).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	oneKeyMs, oneKeyStream := timeConcurrent(ctx, pool, "onekey", goroutines, perGoroutine, func(g, i int) string {
		return "hot-key" // every goroutine hammers the SAME row
	})
	defer func() {
		must(client.Stream[sqlstreams.RawPayload](oneKeyStream).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	time.Sleep(1 * time.Second) // let PG's stats collector flush before reading it
	// the one-hot-key stream's compaction_head table saw only the burst, so
	// its absolute stats are the burst's numbers
	stats := dumpTableStats(ctx, ds, compactionHeadTable(ctx, ds, oneKeyStream))

	total := goroutines * perGoroutine
	fmt.Printf("  %-28s %10.3fms total  %8.4fms/op (%d ops, %d goroutines)\n", "many distinct keys", manyKeysMs, manyKeysMs/float64(total), total, goroutines)
	fmt.Printf("  %-28s %10.3fms total  %8.4fms/op (%d ops, %d goroutines)\n", "one hot key", oneKeyMs, oneKeyMs/float64(total), total, goroutines)
	fmt.Printf("  -> %.1fx slower under full serialization on a single key\n", oneKeyMs/manyKeysMs)

	step("dead-tuple growth from the hot-key burst")
	fmt.Printf("  n_live_tup=%d n_dead_tup=%d n_tup_upd=%d\n", stats.liveTup, stats.deadTup, stats.tupUpd)
	fmt.Printf("  -> %d updates against ONE row produced %d dead tuples, pending autovacuum\n",
		stats.tupUpd, stats.deadTup)
}

// ---- helpers ----

// timeSequential runs n single-threaded publishes, keyFn(i) chosen per call,
// returning total elapsed time in milliseconds.
func timeSequential(ctx context.Context, wpInstance *sqlstreams.ProducerInstance[common.Work], n int, keyFn func(i int) string) float64 {
	start := time.Now()
	for i := range n {
		opts := &sqlstreams.ProduceOptions{}
		if key := keyFn(i); key != "" {
			opts.MessageKey = key
			opts.Compaction = &sqlstreams.CompactionOptions{Enable: true}
		}
		_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
			return common.NewWork(30, "admin@example.com")
		}, opts)
		must(err)
	}
	return float64(time.Since(start).Microseconds()) / 1000.0
}

// timeConcurrent registers its own stream, fires goroutines*perGoroutine
// publishes across `goroutines` concurrent workers, and returns total
// elapsed time plus the stream name (caller destroys it once done reading it).
func timeConcurrent(ctx context.Context, pool *pgxpool.Pool, label string, goroutines, perGoroutine int, keyFn func(g, i int) string) (float64, string) {
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	must(err)

	name := fmt.Sprintf("phase8c.compactionheadwrite.%s.%d", label, time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](name).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: largePartitionSize})
	must(err)

	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	must(err)

	start := time.Now()
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Go(func() {
			for i := range perGoroutine {
				_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
					return common.NewWork(30, "admin@example.com")
				}, &sqlstreams.ProduceOptions{MessageKey: keyFn(g, i), Compaction: &sqlstreams.CompactionOptions{Enable: true}})
				must(err)
			}
		})
	}
	wg.Wait()
	elapsedMs := float64(time.Since(start).Microseconds()) / 1000.0

	return elapsedMs, name
}

type tableStats struct {
	liveTup int64
	deadTup int64
	tupUpd  int64
}

// compactionHeadTable resolves a stream's compaction_head_<id> table name from
// the catalog.
// the name comes back bare: its one reader matches pg_stat_user_tables.relname
func compactionHeadTable(ctx context.Context, ds *iDatastore.PostgresDatastore, streamName string) string {
	var id int64
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.stream_config WHERE name = $1;`, ds.Schema), streamName).Scan(&id))
	return stream.CompactionHeadTable(id)
}

func dumpTableStats(ctx context.Context, ds *iDatastore.PostgresDatastore, table string) tableStats {
	var s tableStats
	sql := `
		SELECT n_live_tup, n_dead_tup, n_tup_upd
		FROM pg_stat_user_tables
		WHERE relname = $1;
	`
	must(ds.Pool.QueryRow(ctx, sql, table).Scan(&s.liveTup, &s.deadTup, &s.tupUpd))
	return s
}

func pctOver(got, baseline float64) float64 {
	if baseline == 0 {
		return 0
	}
	return (got - baseline) / baseline * 100
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
func must(err error) {
	if err != nil {
		die(err.Error())
	}
}
func die(msg string) {
	panic(testFailure{message: msg})
}
