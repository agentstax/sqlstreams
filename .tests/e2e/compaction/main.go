package main

// Log compaction e2e test: latest-per-key filtering at claim time.
//
// Registers its own stream (destroyed on exit), self-seeds, fully
// self-contained -- no dependency on external state.
//
// Confirms, in order:
//   - a claim spanning several versions of a message key only returns the
//     latest; older versions still physically exist in message_log_<id>
//     (filtered at read time, never deleted).
//   - a version superseded AFTER its predecessor already delivered doesn't
//     retroactively unsend that predecessor -- committed only ever moves
//     forward, and the superseded row is still physically present.
//   - the crash/reclaim race directly: a worker claims a keyed row, crashes
//     before Commit, a newer version of that key lands, the lease expires
//     and gets reclaimed -- the reclaimed read now returns NOTHING (the
//     superseded row gets zero delivery, by design), and the newer version
//     still gets its own independent delivery later.
//   - a message whose own payload marks it deleted (a pure application
//     convention, not a framework concept) is still delivered normally on
//     both the CURSOR and LIFECYCLE paths.
//   - EXPLAIN ANALYZE over unkeyed-only rows shows the compaction subplan
//     never executes (the OR short-circuits on compaction_rank IS NULL).

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	cursoradvancerdatastore "github.com/agentstax/sqlstreams/pkg/consume/cursoradvancer/controller/datastore"
	deliveryconsumercontroller "github.com/agentstax/sqlstreams/pkg/consume/deliveryconsumer/controller"
	messageconsumercontroller "github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

const (
	cursorGroup    = "phase8c.compaction.cursor"
	lifecycleGroup = "phase8c.compaction.lifecycle"
)

// KeyedRecord is this e2e test's own payload shape. Deleted is the tombstone
// decision made real: the framework has zero opinion on it, a consumer just
// reads its own field like any other.
type KeyedRecord struct {
	Key     string `json:"key"`
	Version int    `json:"version"`
	Deleted bool   `json:"deleted,omitempty"`
}

func (KeyedRecord) SchemaVersion() int { return 1 }

// set by main from RegisterGroup -- helpers are id-keyed
var cursorGroupID int64

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

	streamName := fmt.Sprintf("phase8c.compaction.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	cd, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)
	messageConsumers, err := messageconsumercontroller.NewMessageConsumerGroupController(ds, ds.Logger)
	common.Must(err)
	deliveryConsumers, err := deliveryconsumercontroller.NewDeliveryConsumerGroupController(ds, ds.Logger)
	common.Must(err)
	cursorAdvancerDatastore, err := cursoradvancerdatastore.NewCursorAdvancerDatastore(ds, ds.Logger)
	common.Must(err)
	wpInstance, err := client.Stream[KeyedRecord](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)
	cursorGroupID = mustGroupID(cd.RegisterGroup(ctx, tp.Id, cursorGroup, consume.Beginning()))

	const lease = 2 * time.Second
	const maxRangeReclaims = 3 // never exhausted in this e2e test -- exactly one reclaim happens

	// ===== latest-per-key survives, older rows stay physically present (ids 1-6) =====
	step("publish 3 versions of user:1, 1 unkeyed row, 2 versions of user:2")
	publish(ctx, wpInstance, "user:1", 1, false) // id 1
	publish(ctx, wpInstance, "user:1", 2, false) // id 2
	publish(ctx, wpInstance, "user:1", 3, false) // id 3 <- latest for user:1
	publish(ctx, wpInstance, "", 0, false)       // id 4, unkeyed -- never compacted
	publish(ctx, wpInstance, "user:2", 1, false) // id 5
	publish(ctx, wpInstance, "user:2", 2, false) // id 6 <- latest for user:2

	claim, err := messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, cursorGroupID, 1, 10, maxRangeReclaims, lease, stream.DeliveryLogModeFailures)
	common.Must(err)
	if claim == nil {
		common.Die("expected a fresh claim, got nil (no work?)")
	}
	fmt.Printf("  claimed (%d,%d]  ids=%v\n", claim.Lease.Low, claim.Lease.High, ids(claim.Messages))
	assertIDs("only the latest version of each key, plus the unkeyed row, come back", ids(claim.Messages), []int64{3, 4, 6})
	assertInt("all 6 rows still physically exist -- compaction filters, never deletes", rowCount(ctx, ds, tp.Id), 6)

	common.Must(messageConsumers.Commit(ctx, tp.Id, cursorGroupID, claim.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures))
	committed := advance(ctx, cursorAdvancerDatastore, tp.Id)
	assertInt("committed advances over the whole range regardless of compaction", committed, 6)

	// ===== a delivered version isn't retroactively unsent once superseded (ids 7-8) =====
	step("user:3 v1 delivered, THEN v2 is published and delivered on its own later read")
	publish(ctx, wpInstance, "user:3", 1, false) // id 7
	claim, err = messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, cursorGroupID, 1, 1, maxRangeReclaims, lease, stream.DeliveryLogModeFailures)
	common.Must(err)
	assertIDs("user:3 v1 delivered -- it's the only version so far", ids(claim.Messages), []int64{7})
	common.Must(messageConsumers.Commit(ctx, tp.Id, cursorGroupID, claim.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures))
	committed = advance(ctx, cursorAdvancerDatastore, tp.Id)
	assertInt("committed", committed, 7)

	publish(ctx, wpInstance, "user:3", 2, false) // id 8, published AFTER v1 already delivered+committed
	claim, err = messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, cursorGroupID, 1, 1, maxRangeReclaims, lease, stream.DeliveryLogModeFailures)
	common.Must(err)
	assertIDs("user:3 v2 delivered on its own read -- v1's earlier delivery is untouched", ids(claim.Messages), []int64{8})
	common.Must(messageConsumers.Commit(ctx, tp.Id, cursorGroupID, claim.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures))
	committed = advance(ctx, cursorAdvancerDatastore, tp.Id)
	assertInt("committed only ever moves forward", committed, 8)
	assertTrue("v1 (id 7) is still physically present -- compaction never rewrites history", rowExists(ctx, ds, tp.Id, 7))

	// ===== the crash/reclaim race (ids 9-10) =====
	step("WORKER 1 claims user:4 v1, then crashes before Commit")
	publish(ctx, wpInstance, "user:4", 1, false) // id 9
	claim1, err := messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, cursorGroupID, 1, 1, maxRangeReclaims, lease, stream.DeliveryLogModeFailures)
	common.Must(err)
	if claim1 == nil {
		common.Die("expected a fresh claim, got nil")
	}
	assertIDs("WORKER 1 claims user:4 v1", ids(claim1.Messages), []int64{9})
	fmt.Printf("  claimed (%d,%d] lease=%s -- WORKER 1 crashes here, never calls Commit\n",
		claim1.Lease.Low, claim1.Lease.High, shortTok(claim1.Lease.Token))

	step("a newer version of user:4 lands while v1 is still (unknowingly) in flight")
	publish(ctx, wpInstance, "user:4", 2, false) // id 10

	step(fmt.Sprintf("sleep %s -- let the crashed lease expire", lease+500*time.Millisecond))
	time.Sleep(lease + 500*time.Millisecond)

	step("WORKER 2 polls: reclaims the exact expired range -- v1 is now superseded")
	claim2, err := messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, cursorGroupID, 1, 1, maxRangeReclaims, lease, stream.DeliveryLogModeFailures)
	common.Must(err)
	if claim2 == nil {
		common.Die("expected a reclaim, got nil")
	}
	assertInt("reclaim re-reads the exact same range low", claim2.Lease.Low, claim1.Lease.Low)
	assertInt("reclaim re-reads the exact same range high", claim2.Lease.High, claim1.Lease.High)
	if shortTok(claim2.Lease.Token) == shortTok(claim1.Lease.Token) {
		common.Die("token was not rotated")
	}
	assertIDs("v1 is superseded -- the reclaimed read returns NOTHING for this range, by design", ids(claim2.Messages), []int64{})
	fmt.Println("  -> the accepted tradeoff: at-least-once is a per-KEY guarantee (the current latest")
	fmt.Println("     value eventually arrives), not a per-message one -- v1 owed nothing further")
	fmt.Println("     once v2 superseded it, exactly like Kafka's own compacted-stream contract")

	common.Must(messageConsumers.Commit(ctx, tp.Id, cursorGroupID, claim2.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures))
	committed = advance(ctx, cursorAdvancerDatastore, tp.Id)
	assertInt("committed moves past the (empty) reclaimed range", committed, 9)

	step("v2 still gets its own, independent delivery -- the obligation carried forward")
	claim3, err := messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, cursorGroupID, 1, 1, maxRangeReclaims, lease, stream.DeliveryLogModeFailures)
	common.Must(err)
	assertIDs("user:4 v2 delivered", ids(claim3.Messages), []int64{10})
	common.Must(messageConsumers.Commit(ctx, tp.Id, cursorGroupID, claim3.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures))
	committed = advance(ctx, cursorAdvancerDatastore, tp.Id)
	assertInt("committed", committed, 10)

	// ===== tombstones are a pure app convention (ids 11-12) =====
	step("a message marked deleted in its OWN payload is delivered normally on both paths")
	publish(ctx, wpInstance, "user:5", 1, true) // id 11, CURSOR path
	claim, err = messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, cursorGroupID, 1, 1, maxRangeReclaims, lease, stream.DeliveryLogModeFailures)
	common.Must(err)
	assertIDs("CURSOR path delivers the deleted-marked message like any other", ids(claim.Messages), []int64{11})
	assertTrue("payload's own Deleted field survives -- the query never special-cases it", decode(claim.Messages[0].Payload).Deleted)
	common.Must(messageConsumers.Commit(ctx, tp.Id, cursorGroupID, claim.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures))
	committed = advance(ctx, cursorAdvancerDatastore, tp.Id)
	assertInt("committed", committed, 11)

	publish(ctx, wpInstance, "user:6", 1, true)                                                        // id 12, LIFECYCLE path
	lifecycleGroupID := mustGroupID(cd.RegisterGroup(ctx, tp.Id, lifecycleGroup, consume.Beginning())) // fresh group scans from mark 0 -> the whole log
	common.Must(deliveryConsumers.FanOut(ctx, tp.Id, lifecycleGroupID, 1, 100))
	delivered, err := deliveryConsumers.ClaimMessagesWithLifecycle(ctx, tp.Id, lifecycleGroupID, 20)
	common.Must(err)
	assertIDs("FanOut applies the identical compaction predicate across its whole scan, not a range",
		deliveryIDs(delivered), []int64{3, 4, 6, 8, 10, 11, 12})
	deletedDelivered := false
	for _, d := range delivered {
		if d.MessageId == 12 {
			deletedDelivered = decode(d.Payload).Deleted
		}
	}
	assertTrue("LIFECYCLE path also delivers the deleted-marked message", deletedDelivered)

	// ===== EXPLAIN: unkeyed-only traffic never pays the compaction subplan =====
	step("EXPLAIN ANALYZE: an unkeyed-only read never executes the compaction subplan")
	for range 5 {
		publish(ctx, wpInstance, "", 0, false) // ids 13-17
	}
	explainNoCompactionSubplan(ctx, ds, tp.Id, 12, 17)

	fmt.Println("\n✅ COMPACTION E2E TEST PASSED")
	fmt.Println("   latest-per-key survives, older rows persist untouched -> a delivered version stays")
	fmt.Println("   delivered even once superseded -> a crashed-then-superseded row gets zero delivery")
	fmt.Println("   while its successor still gets its own -> tombstones are pure app convention on both")
	fmt.Println("   paths -> unkeyed reads never pay the compaction subplan's cost.")
	return nil
}

// ---- helpers ----

func publish(ctx context.Context, wpInstance *sqlstreams.ProducerInstance[KeyedRecord], key string, version int, deleted bool) {
	opts := &sqlstreams.ProduceOptions{}
	if key != "" {
		opts.MessageKey = key
		opts.Compaction = &sqlstreams.CompactionOptions{Enable: true}
	}
	_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*KeyedRecord, error) {
		return &KeyedRecord{Key: key, Version: version, Deleted: deleted}, nil
	}, opts)
	common.Must(err)
}

func advance(ctx context.Context, cursorAdvancerDatastore *cursoradvancerdatastore.CursorAdvancerDatastore, streamId int64) int64 {
	c, err := cursorAdvancerDatastore.AdvanceCommitted(ctx, streamId, cursorGroupID)
	common.Must(err)
	return c
}

func decode(payload json.RawMessage) KeyedRecord {
	var kr KeyedRecord
	common.Must(json.Unmarshal(payload, &kr))
	return kr
}

// explainNoCompactionSubplan EXPLAIN ANALYZEs the exact shape readMessages runs
// over an id range that only contains unkeyed rows, then checks the plan for
// the compaction_head lookup being marked never executed -- proof the OR's left
// disjunct (compaction_rank IS NULL) short-circuited it for every row.
func explainNoCompactionSubplan(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId, low, high int64) {
	logTable := fmt.Sprintf("%s.%s", ds.Schema, stream.MessageLogTable(streamId))
	sql := fmt.Sprintf(`
		EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF) SELECT m.id, m.payload, m.created_at FROM %s m
		WHERE m.id > $1
			AND m.id <= $2
			AND (
				NOT EXISTS (SELECT 1 FROM %s.%s b WHERE b.consumer_group_id = $3)
				OR EXISTS (SELECT 1 FROM %s.%s b WHERE b.consumer_group_id = $3 AND m.routing_key ~ b.pattern_regex)
			)
			AND (
				m.compaction_rank IS NULL
				OR m.id = (SELECT message_id FROM %s.%s
					WHERE compaction_key = m.message_key)
			)
		ORDER BY m.id;
	`, logTable, ds.Schema, stream.BindingConfigTable(streamId), ds.Schema, stream.BindingConfigTable(streamId), ds.Schema, stream.CompactionHeadTable(streamId))

	rows, err := ds.Pool.Query(ctx, sql, low, high, cursorGroupID)
	common.Must(err)
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var line string
		common.Must(rows.Scan(&line))
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	common.Must(rows.Err())
	fmt.Print(plan.String())

	matched, err := regexp.MatchString(`(?i)compaction_head.*never executed`, plan.String())
	common.Must(err)
	assertTrue("the compaction_head lookup never executed against unkeyed-only rows", matched)
}

func rowCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) int64 {
	return scalar(ctx, ds, fmt.Sprintf(`SELECT count(*) FROM %s.%s`, ds.Schema, stream.MessageLogTable(streamId)))
}

func rowExists(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId, id int64) bool {
	return scalar(ctx, ds, fmt.Sprintf(`SELECT count(*) FROM %s.%s WHERE id=$1`, ds.Schema, stream.MessageLogTable(streamId)), id) == 1
}

func scalar(ctx context.Context, ds *iDatastore.PostgresDatastore, q string, args ...any) int64 {
	var v int64
	common.Must(ds.Pool.QueryRow(ctx, q, args...).Scan(&v))
	return v
}

func ids(msgs []messageconsumercontroller.Message) []int64 {
	out := make([]int64, len(msgs))
	for i, m := range msgs {
		out[i] = m.Id
	}
	return out
}

func deliveryIDs(rows []deliveryconsumercontroller.Delivery) []int64 {
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = r.MessageId
	}
	return out
}

func shortTok[T fmt.Stringer](t T) string {
	s := t.String()
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
func assertInt(label string, got, want int64) {
	if got != want {
		common.Die(fmt.Sprintf("%s: got %d, want %d", label, got, want))
	}
	fmt.Printf("  ✓ %s (%d)\n", label, got)
}
func assertIDs(label string, got, want []int64) {
	if len(got) != len(want) {
		common.Die(fmt.Sprintf("%s: got %v, want %v", label, got, want))
	}
	for i := range got {
		if got[i] != want[i] {
			common.Die(fmt.Sprintf("%s: got %v, want %v", label, got, want))
		}
	}
	fmt.Printf("  ✓ %s %v\n", label, got)
}
func assertTrue(label string, cond bool) {
	if !cond {
		common.Die(fmt.Sprintf("%s: got false, want true", label))
	}
	fmt.Printf("  ✓ %s\n", label)
}

func mustGroupID(g *consume.Consumer, err error) int64 { common.Must(err); return g.Id }
