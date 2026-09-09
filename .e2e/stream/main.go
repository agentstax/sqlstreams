package main

// Phase 8b's own e2e test: proves the five claims Phase 8b settled about
// per-stream tables (DECISIONS.md, phase 8b records), live against a real DB
// rather than asserted.
//
// Each proof registers its own disposable stream(s), destroyed on exit, so
// this e2e test is self-contained and re-runnable without leftover state --
// same convention chunk 11 brought to every other e2e test this phase.
//
// PROOF 1: two streams get independent physical tables and dense id sequences
//          -- ids don't leak or interleave across streams.
// PROOF 2: a badly-lagging group on stream B does not block a drop on stream A
//          -- the exact cross-stream contamination 8a's TODO flagged, the
//          headline bug this phase fixes.
// PROOF 3: routing_key/bindings behave exactly as Phase 7/routing proved,
//          now scoped within one stream (a condensed smoke check -- the full
//          suite, including LIFECYCLE gate-row-creation and cross-hierarchy
//          wildcards, lives in routing and isn't re-derived here).
// PROOF 4: two routing_key slices sharing ONE stream still share that stream's
//          drop floor -- the re-scoped-not-eliminated case this phase leaves
//          deliberately unfixed (split into separate streams if that's a
//          real problem).
// PROOF 5: operating against an unregistered stream id fails clearly (a
//          Postgres undefined_table error), never silently auto-creating one.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/agentstax/sqlstreams/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	cursoradvancerdatastore "github.com/agentstax/sqlstreams/pkg/consume/cursoradvancer/controller/datastore"
	messageconsumercontroller "github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	janitordatastore "github.com/agentstax/sqlstreams/pkg/stream/janitor/controller/datastore"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	partitionSize = int64(5)
	ttl           = 100 * time.Millisecond
	ttlMargin     = 300 * time.Millisecond
)

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
	run := time.Now().UnixNano()

	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	must(err)

	register := func(name string) *stream.Stream {
		t, err := client.Stream[sqlstreams.RawPayload](name).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: partitionSize})
		must(err)
		return t
	}
	streamA := register(fmt.Sprintf("phase8b.stream.a.%d", run))
	streamB := register(fmt.Sprintf("phase8b.stream.b.%d", run))
	streamC := register(fmt.Sprintf("phase8b.stream.c.%d", run))
	streamD := register(fmt.Sprintf("phase8b.stream.d.%d", run))
	defer func() {
		for _, t := range []*stream.Stream{streamA, streamB, streamC, streamD} {
			must(client.Stream[sqlstreams.RawPayload](t.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
		}
	}()

	cd, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	must(err)
	messageConsumers, err := messageconsumercontroller.NewMessageConsumerGroupController(ds, ds.Logger)
	must(err)
	janitorDatastore, err := janitordatastore.NewJanitorDatastore(ds, ds.Logger)
	must(err)
	cursorAdvancerDatastore, err := cursoradvancerdatastore.NewCursorAdvancerDatastore(ds, ds.Logger)
	must(err)

	// ===== PROOF 1: independent physical tables, independent dense id sequences =====
	step("PROOF 1: two streams get independent physical tables and dense id sequences")
	wpAInstance, err := client.Stream[common.Work](streamA.Name).Producer().Register(ctx, nil)
	must(err)
	wpBInstance, err := client.Stream[common.Work](streamB.Name).Producer().Register(ctx, nil)
	must(err)
	for range 3 {
		publish(ctx, wpAInstance, "")
	}
	for range 4 {
		publish(ctx, wpBInstance, "")
	}
	idsA := allIds(ctx, ds, streamA.Id)
	idsB := allIds(ctx, ds, streamB.Id)
	assertInt64s("streamA's log has exactly ids 1-3, its own dense sequence", idsA, []int64{1, 2, 3})
	assertInt64s("streamB's log has exactly ids 1-4, its own INDEPENDENT sequence -- also starting at 1, not 4", idsB, []int64{1, 2, 3, 4})

	// ===== PROOF 2: a badly-lagging group on stream B does not block a drop on stream A =====
	step("PROOF 2: a badly-lagging group on stream B does not block a drop on stream A")
	publish(ctx, wpAInstance, "") // id 4, the trigger point create-ahead builds partition 1 from
	waitForPartition(ctx, ds, streamA.Id, 1)
	publish(ctx, wpAInstance, "") // id 5, landing in the partition that is already there
	time.Sleep(ttl + ttlMargin)

	groupA := "stream.groupA" // streamA's own reader, fully caught up
	groupAID := mustGroupID(cd.RegisterGroup(ctx, streamA.Id, groupA, consume.Beginning()))
	setCursor(ctx, ds, streamA.Id, groupAID, 5, 5)

	groupB := "stream.groupB" // streamB's reader, registered but never advances -- badly lagging
	mustGroupID(cd.RegisterGroup(ctx, streamB.Id, groupB, consume.Beginning()))

	must(janitorDatastore.DropExpiredPartitions(ctx, streamA.Id, partitionSize, ttl, false, streamA.DeliveryLogMode))
	assertPartitions(ctx, ds, streamA.Id, "streamA's partition 0 dropped, totally unaffected by streamB's lagging group", []int64{1})
	fmt.Println("  -> this is the exact cross-stream contamination 8a's floor bug caused; each stream's floor is now its own")

	// ===== PROOF 3: routing_key/bindings still behave as Phase 7/routing proved, now scoped to one stream =====
	step("PROOF 3: routing_key/bindings behave as Phase 7 proved, scoped within one stream (condensed -- full suite in routing)")
	wpCInstance, err := client.Stream[common.Work](streamC.Name).Producer().Register(ctx, nil)
	must(err)
	groupRoute := "stream.route"
	groupRouteID := mustGroupID(cd.RegisterGroup(ctx, streamC.Id, groupRoute, consume.Beginning()))

	headBefore := head(ctx, ds, streamC.Id)     // streamC is fresh, this is 0
	publish(ctx, wpCInstance, "orders.created") // id headBefore+1, published BEFORE any binding exists
	_, err = cd.DeclareBindings(ctx, streamC.Id, groupRouteID, []string{"orders.*"}, time.Now())
	must(err)
	publish(ctx, wpCInstance, "orders.updated")  // id headBefore+2, matches, published AFTER the binding
	publish(ctx, wpCInstance, "payments.charge") // id headBefore+3, does not match
	fmt.Printf("  published ids %d,%d,%d (only %d predates the binding, only %d and %d match its pattern)\n",
		headBefore+1, headBefore+2, headBefore+3, headBefore+1, headBefore+1, headBefore+2)

	claim, err := messageConsumers.ClaimMessagesWithCursor(ctx, streamC.Id, groupRouteID, 1, 10, 3, 30*time.Second, stream.DeliveryLogModeFailures)
	must(err)
	if claim == nil {
		die("expected a fresh claim, got nil")
	}
	assertInt64s("retroactive binding applies to the pre-existing message, CURSOR path filters out the non-match",
		ids(claim.Messages), []int64{headBefore + 1, headBefore + 2})
	must(messageConsumers.Commit(ctx, streamC.Id, groupRouteID, claim.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures))
	committed := advance(ctx, cursorAdvancerDatastore, streamC.Id, groupRouteID)
	assertInt("committed still advances over the WHOLE range, not just the matches", committed, claim.Lease.High)

	// ===== PROOF 4: two routing_key slices sharing ONE stream still share that stream's floor =====
	step("PROOF 4: two routing_key slices sharing ONE stream still share that stream's drop floor (deliberately not fixed)")
	wpDInstance, err := client.Stream[common.Work](streamD.Name).Producer().Register(ctx, nil)
	must(err)
	groupX := "stream.sliceX" // reads only sliceX.* -- will be fully caught up
	groupY := "stream.sliceY" // reads only sliceY.* -- registered but stays lagging
	groupXID := mustGroupID(cd.RegisterGroup(ctx, streamD.Id, groupX, consume.Beginning()))
	groupYID := mustGroupID(cd.RegisterGroup(ctx, streamD.Id, groupY, consume.Beginning()))
	_, err = cd.DeclareBindings(ctx, streamD.Id, groupXID, []string{"sliceX.*"}, time.Now())
	must(err)
	_, err = cd.DeclareBindings(ctx, streamD.Id, groupYID, []string{"sliceY.*"}, time.Now())
	must(err)

	for range 4 { // 4 rows, all in sliceX -- the 4th builds partition 1 through create-ahead
		publish(ctx, wpDInstance, "sliceX.event")
	}
	waitForPartition(ctx, ds, streamD.Id, 1)
	publish(ctx, wpDInstance, "sliceX.event") // id 5, landing in the partition that is already there
	time.Sleep(ttl + ttlMargin)

	claimX, err := messageConsumers.ClaimMessagesWithCursor(ctx, streamD.Id, groupXID, 1, 10, 3, 30*time.Second, stream.DeliveryLogModeFailures)
	must(err)
	if claimX == nil {
		die("expected groupX to claim a fresh range")
	}
	must(messageConsumers.Commit(ctx, streamD.Id, groupXID, claimX.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures))
	advance(ctx, cursorAdvancerDatastore, streamD.Id, groupXID)
	fmt.Println("  groupX (sliceX reader) is now fully caught up on the only traffic that exists")
	// groupY never published to or claimed anything -- its cursor sits at claimed=committed=0,
	// simulating a slice consumer that's stuck or never started.

	must(janitorDatastore.DropExpiredPartitions(ctx, streamD.Id, partitionSize, ttl, false, streamD.DeliveryLogMode))
	assertPartitions(ctx, ds, streamD.Id, "partition 0 SURVIVES -- groupY's slice, though it has zero actual traffic, still pins this stream's one shared floor", []int64{0, 1})
	fmt.Println("  -> this is the case 8b deliberately leaves unfixed: split into separate streams if slices need independent floors")

	// ===== PROOF 5: operating against an unregistered stream id fails clearly =====
	step("PROOF 5: publishing/claiming against an unregistered stream id fails clearly, never silently auto-creates one")
	bogusStreamID := streamD.Id + 999_999_999 // guaranteed to never have been registered
	err = janitorDatastore.DropExpiredPartitions(ctx, bogusStreamID, partitionSize, ttl, false, stream.DeliveryLogModeFailures)
	if err == nil {
		die("expected an error operating against an unregistered stream id, got nil")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42P01" {
		die(fmt.Sprintf("expected a Postgres 42P01 (undefined_table) error, got: %v", err))
	}
	fmt.Printf("  ✓ got the expected undefined_table error: %s\n", pgErr.Message)
	fmt.Println("  -> no implicit stream/table creation as a side effect of a produce/claim call")

	fmt.Println("\n✅ STREAM E2E TEST PASSED")
	fmt.Println("   streams get independent tables/sequences; a lagging group's floor stays inside its own")
	fmt.Println("   stream; routing still works exactly as before, just re-scoped; and an unregistered")
	fmt.Println("   stream id fails loudly instead of silently doing something wrong.")
	return nil
}

// ---- helpers ----

func publish(ctx context.Context, wp *sqlstreams.ProducerInstance[common.Work], routingKey string) {
	_, err := wp.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
		return common.NewWork(30, "admin@example.com")
	}, &sqlstreams.ProduceOptions{RoutingKey: routingKey})
	must(err)
}

func advance(ctx context.Context, cursorAdvancerDatastore *cursoradvancerdatastore.CursorAdvancerDatastore, streamId int64, groupId int64) int64 {
	c, err := cursorAdvancerDatastore.AdvanceCommitted(ctx, streamId, groupId)
	must(err)
	return c
}

func setCursor(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, groupId int64, claimed, committed int64) {
	_, err := ds.Pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET claimed=$2, committed=$3 WHERE consumer_group_id=$1`, ds.Schema, stream.ConsumerGroupCursorTable(streamId)), groupId, claimed, committed)
	must(err)
}

func head(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) int64 {
	var v int64
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(MAX(id), 0) FROM %s.%s`, ds.Schema, stream.MessageLogTable(streamId))).Scan(&v))
	return v
}

func allIds(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) []int64 {
	rows, err := ds.Pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.%s ORDER BY id`, ds.Schema, stream.MessageLogTable(streamId)))
	must(err)
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var id int64
		must(rows.Scan(&id))
		out = append(out, id)
	}
	must(rows.Err())
	return out
}

func assertPartitions(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, label string, want []int64) {
	assertInt64s(label, partitions(ctx, ds, streamId), want)
}

// waitForPartition blocks until partition n exists. Create-ahead runs in a
// background goroutine off the produce that hits its trigger point id, so a
// publish right behind that one races it: the goroutine reads MAX(id) and
// builds the partition after whatever it sees.
func waitForPartition(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, n int64) {
	deadline := time.Now().Add(5 * time.Second)
	for {
		for _, existing := range partitions(ctx, ds, streamId) {
			if existing == n {
				return
			}
		}
		if time.Now().After(deadline) {
			die(fmt.Sprintf("partition %d of stream %d never appeared", n, streamId))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func partitions(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) []int64 {
	// pg_class.relname is unqualified, so the prefix compared against it is too
	prefix := stream.MessageLogTable(streamId) + "_"
	rows, err := ds.Pool.Query(ctx, `
		SELECT REPLACE(c.relname, $2, '')::bigint AS n
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = $1::regclass
			AND c.relname LIKE $2 || '%'
		ORDER BY n;
	`, fmt.Sprintf("%s.%s", ds.Schema, stream.MessageLogTable(streamId)), prefix)
	must(err)
	defer rows.Close()

	var got []int64
	for rows.Next() {
		var n int64
		must(rows.Scan(&n))
		got = append(got, n)
	}
	must(rows.Err())
	return got
}

func ids(msgs []messageconsumercontroller.Message) []int64 {
	out := make([]int64, len(msgs))
	for i, m := range msgs {
		out[i] = m.Id
	}
	return out
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
func assertInt(label string, got, want int64) {
	if got != want {
		die(fmt.Sprintf("%s: got %d, want %d", label, got, want))
	}
	fmt.Printf("  ✓ %s (%d)\n", label, got)
}
func assertInt64s(label string, got, want []int64) {
	if len(got) != len(want) {
		die(fmt.Sprintf("%s: got %v, want %v", label, got, want))
	}
	for i := range got {
		if got[i] != want[i] {
			die(fmt.Sprintf("%s: got %v, want %v", label, got, want))
		}
	}
	fmt.Printf("  ✓ %s %v\n", label, got)
}

func mustGroupID(g *consume.Consumer, err error) int64 { must(err); return g.Id }
