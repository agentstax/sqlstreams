package main

// idempotency_key concurrency e2e test: the permanent regression counterpart to
// the throwaway pgxpool test that verified this during the idempotency_key
// per-stream redesign -- every other idempotency e2e test only ever publishes
// sequentially, so the claim+insert CTE's true concurrent behavior (as
// opposed to sequential "retries") has never been exercised as a standing
// test. Mirrors compactionheadrace's concurrent-race precedent.
//
// Two scenarios:
//   - sameKeyConcurrentScenario: N goroutines publish under the SAME
//     idempotency key at once -- exactly 1 must land, regardless of which
//     goroutine's claim insert happened to commit first.
//   - distinctKeysConcurrentScenario: N goroutines each publish under their
//     OWN distinct key, all at once -- every one must land; concurrency
//     alone must never cause a spurious collision or a lost write across
//     unrelated keys.

import (
	"context"
	"fmt"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() (err error) {
	defer common.Recover(&err)
	ctx := context.Background()

	pool, err := common.NewPool(ctx, &sqlstreams.PostgresConnectionConfig{
		MaxConns: 60, // headroom above both scenarios' 50 concurrent publishers
	})
	common.Must(err)
	defer pool.Close()

	sameKeyConcurrentScenario(ctx, pool)
	distinctKeysConcurrentScenario(ctx, pool)

	fmt.Println("\n✅ IDEMPOTENCY KEYS RACE E2E TEST PASSED")
	fmt.Println("   N concurrent publishes under one shared key land exactly once, and N")
	fmt.Println("   concurrent publishes under N distinct keys all land -- the claim+insert")
	fmt.Println("   CTE holds up under true concurrency, not just sequential retries.")
	return nil
}

// sameKeyConcurrentScenario: N goroutines share ONE idempotency key and
// publish at the exact same time -- exactly 1 message and 1 claim row must
// land, however the goroutines' claim inserts happen to interleave/commit.
func sameKeyConcurrentScenario(ctx context.Context, pool *pgxpool.Pool) {
	step("same key, concurrent: N goroutines sharing one idempotency key must land exactly once")

	const n = 50
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)

	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	streamName := fmt.Sprintf("phase9.idempotencykeysrace.same.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: 1000})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)

	key := uuid.NewV7().String()

	var wg sync.WaitGroup
	var duplicateCount atomic.Int64
	for range n {
		wg.Go(func() {
			produced, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
				return common.NewWork(30, "admin@example.com")
			}, &sqlstreams.ProduceOptions{IdempotencyKey: key})
			common.Must(err)
			if produced.Duplicate {
				duplicateCount.Add(1)
			}
		})
	}
	wg.Wait()

	if duplicateCount.Load() != n-1 {
		common.Die(fmt.Sprintf("%d of %d calls reported Duplicate, want %d -- exactly 1 winner", duplicateCount.Load(), n, n-1))
	}
	fmt.Printf("  ✓ exactly 1 of %d concurrent calls stored the message, %d reported Duplicate\n", n, n-1)
	assertCount(ctx, ds, fmt.Sprintf("%s.%s", ds.Schema, stream.MessageLogTable(tp.Id)), 1, fmt.Sprintf("%d concurrent publishes under one shared key landed exactly 1 message", n))
	assertCount(ctx, ds, fmt.Sprintf("%s.%s", ds.Schema, stream.IdempotencyKeyTable(tp.Id)), 1, fmt.Sprintf("%d concurrent publishes under one shared key left exactly 1 claim row", n))

	var exists bool
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.%s WHERE idempotency_key = $1);`, ds.Schema, stream.IdempotencyKeyTable(tp.Id)), key).Scan(&exists))
	if !exists {
		common.Die("the one surviving claim row is not keyed to the idempotency key every goroutine shared")
	}
	fmt.Println("  ✓ the surviving claim row is keyed to the shared idempotency key")
}

// distinctKeysConcurrentScenario: N goroutines each publish under their OWN
// distinct key, all at once -- concurrency alone must never drop a write or
// cause a false collision across unrelated keys.
func distinctKeysConcurrentScenario(ctx context.Context, pool *pgxpool.Pool) {
	step("distinct keys, concurrent: N goroutines each with their own key must all land")

	const n = 50
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)

	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	streamName := fmt.Sprintf("phase9.idempotencykeysrace.distinct.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: 1000})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			key := uuid.NewV7().String()
			_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
				return common.NewWork(30, "admin@example.com")
			}, &sqlstreams.ProduceOptions{IdempotencyKey: key})
			common.Must(err)
		})
	}
	wg.Wait()

	assertCount(ctx, ds, fmt.Sprintf("%s.%s", ds.Schema, stream.MessageLogTable(tp.Id)), n, fmt.Sprintf("%d concurrent publishes under %d distinct keys all landed", n, n))
	assertCount(ctx, ds, fmt.Sprintf("%s.%s", ds.Schema, stream.IdempotencyKeyTable(tp.Id)), n, fmt.Sprintf("%d concurrent publishes under %d distinct keys left %d distinct claim rows", n, n, n))
}

// ---- helpers ----

func assertCount(ctx context.Context, ds *iDatastore.PostgresDatastore, table string, want int, label string) {
	var count int
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s;`, table)).Scan(&count))
	if count != want {
		common.Die(fmt.Sprintf("%s: %s has %d rows, want %d", label, table, count, want))
	}
	fmt.Printf("  ✓ %s (%d)\n", label, count)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
