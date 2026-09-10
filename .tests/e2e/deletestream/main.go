package main

// DeleteStream cascade e2e test: confirms Destroy doesn't just drop message_log and
// the stream row -- the row delete must cascade the stream's consumer groups
// (and their cursor/binding), its maintenance duties and migration history,
// while the FK-less state (leases, compaction_head) is deleted around it and
// the per-stream delivery_<id>/delivery_log_<id>/idempotency_key_<id> tables
// are dropped outright, or that state is permanently orphaned (nothing else
// ever deletes it).
//
// Seeds one row in each of the shared tables plus the per-stream delivery and
// idempotency_key tables via the real datastore methods, deliberately
// leaving a lease OPEN and a delivery row unclaimed -- the messiest state a
// stream could be destroyed in mid-flight, not a conveniently-already-resolved
// one. Also records one failed lifecycle attempt so delivery_log_<id> is
// exercised and confirmed dropped outright, same as delivery_<id>.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	deliveryconsumercontroller "github.com/agentstax/sqlstreams/pkg/consume/deliveryconsumer/controller"
	messageconsumercontroller "github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

const group = "phase9.deletestream.group"

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

	streamName := fmt.Sprintf("phase9.deletestream.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: 1000})
	must(err)

	cd, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	must(err)
	messageConsumers, err := messageconsumercontroller.NewMessageConsumerGroupController(ds, ds.Logger)
	must(err)
	deliveryConsumers, err := deliveryconsumercontroller.NewDeliveryConsumerGroupController(ds, ds.Logger)
	must(err)
	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	must(err)

	step("seed a row in every stream-scoped table")

	groupId := mustGroupID(cd.RegisterGroup(ctx, tp.Id, group, consume.Beginning()))
	_, err = cd.DeclareBindings(ctx, tp.Id, groupId, []string{"orders.*"}, time.Now())
	must(err)

	fn := func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
		return common.NewWork(30, "admin@example.com")
	}
	// Compaction seeds compaction_head; the default (protected) idempotency
	// claim seeds idempotency_key -- one Produce call, two tables.
	_, err = wpInstance.ProduceFunc(ctx, fn, &sqlstreams.ProduceOptions{RoutingKey: "orders.created", MessageKey: "seed-key", Compaction: &sqlstreams.CompactionOptions{Enable: true}})
	must(err)

	claim, err := messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, 10, 3, 5*time.Second, stream.DeliveryLogModeFailures)
	must(err)
	if claim == nil {
		die("expected a claim, got nil")
	}
	// deliberately never Commit -- leaves the lease open

	must(deliveryConsumers.FanOut(ctx, tp.Id, groupId, 1, 100)) // materializes a 'ready' delivery row, left unclaimed

	// claim it via the lifecycle path and fail it once -- status flips
	// ready->inflight->ready in place (still 1 delivery row) while writing one
	// delivery_log row, without touching cursor/lease (lifecycle path skips both).
	claimedLifecycle, err := deliveryConsumers.ClaimMessagesWithLifecycle(ctx, tp.Id, groupId, 10)
	must(err)
	if len(claimedLifecycle) != 1 {
		die(fmt.Sprintf("expected 1 lifecycle claim, got %d", len(claimedLifecycle)))
	}
	must(deliveryConsumers.RecordFailure(ctx, 3, &claimedLifecycle[0], errors.New("seed failure"), tp.DeliveryLogMode))

	for _, table := range []string{"consumer_group_cursor", "claim_lease", "binding_config"} {
		assertGroupRowCount(ctx, ds, fmt.Sprintf("%s.%s_%d", ds.Schema, table, tp.Id), groupId, 1, "before Destroy")
	}
	assertCompactionHeadCount(ctx, ds, tp.Id, 1, "before Destroy")
	assertTableExists(ctx, ds, fmt.Sprintf("%s.%s", ds.Schema, stream.MessageLogTable(tp.Id)), true)
	assertDeliveryRowCount(ctx, ds, tp.Id, 1, "before Destroy")
	assertTableExists(ctx, ds, fmt.Sprintf("%s.%s", ds.Schema, stream.DeliveryLogTable(tp.Id)), true)
	assertDeliveryLogRowCount(ctx, ds, tp.Id, 1, "before Destroy")
	assertTableExists(ctx, ds, fmt.Sprintf("%s.%s", ds.Schema, stream.IdempotencyKeyTable(tp.Id)), true)
	assertIdempotencyKeyRowCount(ctx, ds, tp.Id, 1, "before Destroy")

	step("Destroy the stream")
	must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))

	assertGroupGone(ctx, ds, groupId)
	for _, table := range []string{
		"message_log", "exception_queue", "delivery_log", "idempotency_key",
		"consumer_group_cursor", "claim_lease", "message_key_lease", "compaction_head", "binding_config", "binding_config_log",
	} {
		assertTableExists(ctx, ds, fmt.Sprintf("%s.%s_%d", ds.Schema, table, tp.Id), false)
	}

	fmt.Println("\n✅ DELETE STREAM CASCADE E2E TEST PASSED")
	fmt.Println("   the stream's groups die with it via the stream_id FK cascade, and all ten")
	fmt.Println("   per-stream tables are dropped outright -- neither the still-open lease nor")
	fmt.Println("   the unclaimed delivery row survive.")
	return nil
}

// ---- helpers ----

func assertGroupRowCount(ctx context.Context, ds *iDatastore.PostgresDatastore, table string, groupId int64, want int, when string) {
	var count int
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE consumer_group_id = $1;`, table), groupId).Scan(&count))
	if count != want {
		die(fmt.Sprintf("%s[group %d] has %d rows %s, want %d", table, groupId, count, when, want))
	}
	fmt.Printf("  ✓ %s has %d row(s) %s\n", table, count, when)
}

func assertCompactionHeadCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, want int, when string) {
	var count int
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s;`, ds.Schema, stream.CompactionHeadTable(streamId))).Scan(&count))
	if count != want {
		die(fmt.Sprintf("compaction_head[stream %d] has %d rows %s, want %d", streamId, count, when, want))
	}
	fmt.Printf("  ✓ compaction_head has %d row(s) %s\n", count, when)
}

// the stream's groups are destroyed WITH it, via the stream_id FK cascade.
func assertGroupGone(ctx context.Context, ds *iDatastore.PostgresDatastore, groupId int64) {
	var rows int
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.consumer_group_config WHERE id = $1;`, ds.Schema), groupId).Scan(&rows))
	if rows != 0 {
		die(fmt.Sprintf("consumer_group %d survived its stream's Destroy", groupId))
	}
	fmt.Printf("  ✓ the stream's group destroyed with it\n")
}

// assertDeliveryRowCount counts delivery_<streamId>'s rows directly -- unlike
// scopedTables, this table has no stream_id column to filter by (it's implicit
// in the table name), so it can't go through assertRowCount's generic form.
func assertDeliveryRowCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, want int, when string) {
	var count int
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s;`, ds.Schema, stream.ExceptionQueueTable(streamId))).Scan(&count))
	if count != want {
		die(fmt.Sprintf("%s.%s has %d rows %s, want %d", ds.Schema, stream.ExceptionQueueTable(streamId), count, when, want))
	}
	fmt.Printf("  ✓ exception_queue_%d has %d row(s) %s\n", streamId, count, when)
}

// assertDeliveryLogRowCount counts delivery_log_<streamId>'s rows directly --
// same no-stream_id-column reason as assertDeliveryRowCount.
func assertDeliveryLogRowCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, want int, when string) {
	var count int
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s;`, ds.Schema, stream.DeliveryLogTable(streamId))).Scan(&count))
	if count != want {
		die(fmt.Sprintf("%s.%s has %d rows %s, want %d", ds.Schema, stream.DeliveryLogTable(streamId), count, when, want))
	}
	fmt.Printf("  ✓ delivery_log_%d has %d row(s) %s\n", streamId, count, when)
}

// assertIdempotencyKeyRowCount counts idempotency_key_<streamId>'s rows
// directly -- same no-stream_id-column reason as assertDeliveryRowCount.
func assertIdempotencyKeyRowCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, want int, when string) {
	var count int
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s;`, ds.Schema, stream.IdempotencyKeyTable(streamId))).Scan(&count))
	if count != want {
		die(fmt.Sprintf("%s.%s has %d rows %s, want %d", ds.Schema, stream.IdempotencyKeyTable(streamId), count, when, want))
	}
	fmt.Printf("  ✓ idempotency_key_%d has %d row(s) %s\n", streamId, count, when)
}

func assertTableExists(ctx context.Context, ds *iDatastore.PostgresDatastore, table string, want bool) {
	var exists *string
	must(ds.Pool.QueryRow(ctx, `SELECT to_regclass($1)::text;`, table).Scan(&exists))
	got := exists != nil
	if got != want {
		die(fmt.Sprintf("%s exists=%v, want %v", table, got, want))
	}
	fmt.Printf("  ✓ %s exists=%v\n", table, got)
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

func mustGroupID(g *consume.Consumer, err error) int64 { must(err); return g.Id }
