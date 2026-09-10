package main

// multi-target transactional enqueue e2e test: does sqlstreams.InTransaction +
// Producer.ProduceInTx actually deliver the atomicity/isolation
// guarantees the design promises?
//
// Four scenarios:
//   - atomicPublishScenario: two targets published inside one InTransaction
//     closure both land together on success.
//   - rollbackOnFailureScenario: the second target's ProduceFuncInTx closure
//     returning an error rolls back the WHOLE transaction, not just that
//     target -- the first target's insert never lands either.
//   - partitionSelfHealIsolationScenario: forcing a missing-partition retry
//     on the second target must not touch the first target's already-made
//     insert, and must not rerun a caller side effect that already fired
//     between the two ProduceInTx calls.
//   - ambiguousCommitScenario: a Commit-time failure (a deferred FK
//     violation, so it surfaces at Commit, not at any INSERT) comes back
//     from InTransaction completely unclassified -- no diagnostic.DiagnosticError
//     wrapping, no special-casing. InTransaction never retries; whether a
//     rerun is safe is the caller's call, so the raw error must reach them.
//   - callerKeyRetryScenario: the sanctioned way to make that rerun safe --
//     caller-supplied IdempotencyKeys per target. Rerunning the whole
//     closure under the same keys (a retry after a lost commit confirmation) dedups
//     every target instead of double-publishing.

import (
	"context"
	"errors"
	"fmt"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"os"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/jackc/pgx/v5/pgconn"
)

var fn = func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
	return common.NewWork(30, "admin@example.com")
}

// work is what fn returns, for the value-taking verbs.
func work() *common.Work {
	built, err := common.NewWork(30, "admin@example.com")
	must(err)
	return built
}

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

	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	must(err)

	atomicPublishScenario(ctx, client, ds)
	rollbackOnFailureScenario(ctx, client, ds)
	partitionSelfHealIsolationScenario(ctx, client, ds)
	ambiguousCommitScenario(ctx, client, ds)
	callerKeyRetryScenario(ctx, client, ds)

	fmt.Println("\n✅ MULTI-TARGET E2E TEST PASSED")
	fmt.Println("   two targets in one InTransaction closure commit together, a failure on")
	fmt.Println("   either rolls back both, a missing-partition self-heal on one target")
	fmt.Println("   never touches the other's work or reruns a side effect between them, a")
	fmt.Println("   Commit-time failure surfaces completely unclassified, and rerunning the")
	fmt.Println("   closure under caller-supplied keys dedups instead of double-publishing.")
	return nil
}

func atomicPublishScenario(ctx context.Context, client *sqlstreams.Client, ds *iDatastore.PostgresDatastore) {
	step("atomic publish: two targets in one InTransaction closure both land together")

	streamA, wpA, cleanupA := newTarget(ctx, client, "a", 1000)
	defer cleanupA()
	streamB, wpB, cleanupB := newTarget(ctx, client, "b", 1000)
	defer cleanupB()

	err := client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
		if _, err := wpA.ProduceInTx(ctx, tx, work(), nil); err != nil {
			return err
		}
		_, err := wpB.ProduceInTx(ctx, tx, work(), nil)
		return err
	})
	must(err)

	assertMessageLogCount(ctx, ds, streamA.Id, 1)
	assertMessageLogCount(ctx, ds, streamB.Id, 1)
	fmt.Println("  ✓ both targets committed together")
}

func rollbackOnFailureScenario(ctx context.Context, client *sqlstreams.Client, ds *iDatastore.PostgresDatastore) {
	step("rollback on failure: second target's producerFunc erroring rolls back BOTH, not just itself")

	streamA, wpA, cleanupA := newTarget(ctx, client, "a", 1000)
	defer cleanupA()
	streamB, wpB, cleanupB := newTarget(ctx, client, "b", 1000)
	defer cleanupB()

	wantErr := errors.New("second target refuses to publish")
	err := client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
		if _, err := wpA.ProduceInTx(ctx, tx, work(), nil); err != nil {
			return err
		}
		_, err := wpB.ProduceFuncInTx(ctx, tx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
			return nil, wantErr
		}, nil)
		return err
	})
	if !errors.Is(err, wantErr) {
		die(fmt.Sprintf("InTransaction returned %v, want %v surfaced as-is", err, wantErr))
	}

	assertMessageLogCount(ctx, ds, streamA.Id, 0)
	assertMessageLogCount(ctx, ds, streamB.Id, 0)
	fmt.Println("  ✓ target A's insert never lands either -- one shared tx, not two independent publishes")
}

func partitionSelfHealIsolationScenario(ctx context.Context, client *sqlstreams.Client, ds *iDatastore.PostgresDatastore) {
	step("partition self-heal isolation: B's internal retry must not touch A's work or rerun a side effect between them")

	streamA, wpA, cleanupA := newTarget(ctx, client, "a", 1000)
	defer cleanupA()
	// partitionSize=2 -- one seeded row fills partition_0 [0,2) exactly
	// (BIGSERIAL starts at 1), so the NEXT id has nowhere to land yet.
	streamB, wpB, cleanupB := newTarget(ctx, client, "b", 2)
	defer cleanupB()

	_, err := wpB.ProduceFunc(ctx, fn, nil)
	must(err)
	assertMessageLogCount(ctx, ds, streamB.Id, 1)

	betweenCalls := 0
	err = client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
		if _, err := wpA.ProduceInTx(ctx, tx, work(), nil); err != nil {
			return err
		}
		betweenCalls++                                  // stands in for a caller side effect like sendEmailConfirmation
		_, err := wpB.ProduceInTx(ctx, tx, work(), nil) // misses its partition, self-heals
		return err
	})
	must(err)

	if betweenCalls != 1 {
		die(fmt.Sprintf("side effect between targets fired %d times, want exactly 1 -- B's self-heal retry must not rerun anything before it", betweenCalls))
	}
	assertMessageLogCount(ctx, ds, streamA.Id, 1)
	assertMessageLogCount(ctx, ds, streamB.Id, 2) // 1 seeded + 1 self-healed into a fresh partition
	fmt.Println("  ✓ A's insert survives untouched, the side effect between calls fired exactly once, B self-healed and landed")
}

func ambiguousCommitScenario(ctx context.Context, client *sqlstreams.Client, ds *iDatastore.PostgresDatastore) {
	step("ambiguous commit: a Commit-time failure surfaces unclassified -- retrying is the caller's decision")

	setupDeferredFKFixture(ctx, ds)
	defer teardownDeferredFKFixture(ctx, ds)

	streamA, wpA, cleanupA := newTarget(ctx, client, "a", 1000)
	defer cleanupA()
	streamB, wpB, cleanupB := newTarget(ctx, client, "b", 1000)
	defer cleanupB()

	err := client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
		if _, err := wpA.ProduceInTx(ctx, tx, work(), nil); err != nil {
			return err
		}
		if _, err := wpB.ProduceInTx(ctx, tx, work(), nil); err != nil {
			return err
		}
		// passes now (deferred) -- fails when Commit checks the constraint
		_, err := tx.Exec(ctx, "INSERT INTO multitarget_deferred_child (parent_id) VALUES (-1);")
		return err
	})

	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok || pgErr.Code != "23503" {
		die(fmt.Sprintf("expected the raw foreign_key_violation (23503) from tx.Commit, got %v", err))
	}
	if _, ok := errors.AsType[*diagnostic.DiagnosticError](err); ok {
		die("InTransaction wrapped the commit error in an diagnostic.DiagnosticError -- it must never classify, only surface as-is")
	}

	assertMessageLogCount(ctx, ds, streamA.Id, 0)
	assertMessageLogCount(ctx, ds, streamB.Id, 0)

	fmt.Println("  ✓ Commit-time failure surfaces as the raw driver error, unclassified")
}

// callerKeyRetryScenario: reruns the whole closure under the SAME caller
// keys -- what a caller does after losing the commit confirmation. Auto-minted keys
// resolve fresh per call, so THIS dedup guarantee belongs to caller keys
// alone: without them a closure rerun double-publishes every target.
func callerKeyRetryScenario(ctx context.Context, client *sqlstreams.Client, ds *iDatastore.PostgresDatastore) {
	step("caller-key retry: rerunning the closure under the same keys dedups every target")

	streamA, wpA, cleanupA := newTarget(ctx, client, "a", 1000)
	defer cleanupA()
	streamB, wpB, cleanupB := newTarget(ctx, client, "b", 1000)
	defer cleanupB()

	keyA := uuid.NewV7().String()
	keyB := uuid.NewV7().String()

	closure := func(ctx context.Context, tx sqlstreams.Tx) error {
		if _, err := wpA.ProduceInTx(ctx, tx, work(), &sqlstreams.ProduceOptions{IdempotencyKey: keyA}); err != nil {
			return err
		}
		_, err := wpB.ProduceInTx(ctx, tx, work(), &sqlstreams.ProduceOptions{IdempotencyKey: keyB})
		return err
	}

	must(client.InTransaction(ctx, closure)) // the publish whose confirmation was "lost"
	must(client.InTransaction(ctx, closure)) // the caller's retry

	assertMessageLogCount(ctx, ds, streamA.Id, 1)
	assertMessageLogCount(ctx, ds, streamB.Id, 1)
	fmt.Println("  ✓ both targets landed exactly once across two full closure runs")
}

// ---- fixtures ----

func newTarget(ctx context.Context, client *sqlstreams.Client, label string, partitionSize int64) (*sqlstreams.Stream, *sqlstreams.ProducerInstance[common.Work], func()) {
	name := fmt.Sprintf("multitarget.%s.%d", label, time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](name).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: partitionSize})
	must(err)

	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	must(err)
	return tp, wpInstance, func() {
		must(client.Stream[sqlstreams.RawPayload](name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}
}

// setupDeferredFKFixture builds a scratch FK relationship whose violation is
// only checked at COMMIT, not at INSERT -- the only way to force a genuine
// Commit-time failure on demand.
func setupDeferredFKFixture(ctx context.Context, ds *iDatastore.PostgresDatastore) {
	must(exec(ctx, ds, `CREATE TABLE IF NOT EXISTS multitarget_deferred_parent (id BIGINT PRIMARY KEY);`))
	must(exec(ctx, ds, `
		CREATE TABLE IF NOT EXISTS multitarget_deferred_child (
			id BIGSERIAL PRIMARY KEY,
			parent_id BIGINT NOT NULL,
			CONSTRAINT multitarget_fk FOREIGN KEY (parent_id)
				REFERENCES multitarget_deferred_parent(id) DEFERRABLE INITIALLY DEFERRED
		);
	`))
}

func teardownDeferredFKFixture(ctx context.Context, ds *iDatastore.PostgresDatastore) {
	must(exec(ctx, ds, `DROP TABLE IF EXISTS multitarget_deferred_child;`))
	must(exec(ctx, ds, `DROP TABLE IF EXISTS multitarget_deferred_parent;`))
}

func exec(ctx context.Context, ds *iDatastore.PostgresDatastore, sql string) error {
	_, err := ds.Pool.Exec(ctx, sql)
	return err
}

// ---- helpers ----

func assertMessageLogCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, want int) {
	var count int
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s;`, ds.Schema, stream.MessageLogTable(streamId))).Scan(&count))
	if count != want {
		die(fmt.Sprintf("%s.%s has %d rows, want %d", ds.Schema, stream.MessageLogTable(streamId), count, want))
	}
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
