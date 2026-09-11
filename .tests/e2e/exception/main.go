package main

// Phase 6.5c e2e test: watch committed pin on a failing message, then jump past it.
//
// Registers its own stream (destroyed on exit) and seeds it with 20 messages,
// so the e2e test is fully self-contained -- no dependency on a pre-seeded shared
// message_log the way the pre-8b version needed (`just produce 20` first).
//
// Drives the real datastore methods directly (Commit, AdvanceCommitted,
// ClaimExceptions, RecordExceptionSuccess) so the pin/jump is deterministic and
// asserted on exact cursor state, not inferred from timing.
//
// Confirms: an unresolved exception pins committed below it even while LATER ranges
// keep claiming and committing fine (the exception window never blocks fresh
// range claims), and once the exception resolves, committed jumps straight past
// it to catch up with claimed.

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	cursoradvancerdatastore "github.com/agentstax/sqlstreams/pkg/consume/cursoradvancer/controller/datastore"
	exceptionconsumercontroller "github.com/agentstax/sqlstreams/pkg/consume/exceptionconsumer/controller"
	messageconsumercontroller "github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

const (
	group    = "phase65c.e2e"
	seedRows = 20
)

// set by main from RegisterGroup -- helpers are id-keyed
var groupId int64

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

	streamName := fmt.Sprintf("phase65c.exception.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	cd, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)
	messageConsumers, err := messageconsumercontroller.NewMessageConsumerGroupController(ds, ds.Logger)
	common.Must(err)
	exceptionConsumers, err := exceptionconsumercontroller.NewExceptionConsumerGroupController(ds, ds.Logger)
	common.Must(err)
	cursorAdvancerDatastore, err := cursoradvancerdatastore.NewCursorAdvancerDatastore(ds, ds.Logger)
	common.Must(err)
	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)

	groupId = mustGroupID(cd.RegisterGroup(ctx, tp.Id, group, consume.Beginning()))
	for range seedRows {
		_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
			return common.NewWork(30, "admin@example.com")
		}, nil)
		common.Must(err)
	}
	head := scalar(ctx, ds, fmt.Sprintf(`SELECT COALESCE(max(id),0) FROM %s.%s`, ds.Schema, stream.MessageLogTable(tp.Id)))
	fmt.Printf("stream=%q id=%d message_log head = %d, group = %q\n", streamName, tp.Id, head, group)

	const lease = 5 * time.Second
	const batch = 5
	const maxRangeReclaims = 3 // never hit in this e2e test -- no crashed/reclaimed ranges here

	// ===== range 1: message 3 fails, the rest succeed =====
	step("claim range 1 (ids 1-5), message 3 fails processing")
	claim1, err := messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, batch, maxRangeReclaims, lease, stream.DeliveryLogModeFailures)
	common.Must(err)
	if claim1 == nil {
		common.Die("expected a fresh claim, got nil (no work?)")
	}
	fmt.Printf("  claimed (%d,%d]  ids=%v\n", claim1.Lease.Low, claim1.Lease.High, ids(claim1.Messages))

	const failingId = int64(3)
	exceptions := []messageconsumercontroller.MessageOutcome{{MessageId: failingId, Kind: messageconsumercontroller.OutcomeException, Err: "simulated processing failure"}}
	common.Must(messageConsumers.Commit(ctx, tp.Id, groupId, claim1.Lease.Token, exceptions, 5*time.Second, stream.DeliveryLogModeFailures))
	assertInt64("one unresolved exception", deliveries(ctx, ds, tp.Id), 1)

	committed := advance(ctx, cursorAdvancerDatastore, tp.Id)
	fmt.Printf("  roller tick -> committed = %d\n", committed)
	assertInt64("committed pins below the failing message", committedCol(ctx, ds, tp.Id), failingId-1)

	// ===== range 2: fully succeeds, but committed stays pinned on message 3 =====
	step("claim + commit range 2 (ids 6-10), all succeed")
	claim2, err := messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, batch, maxRangeReclaims, lease, stream.DeliveryLogModeFailures)
	common.Must(err)
	if claim2 == nil {
		common.Die("expected a fresh claim, got nil")
	}
	common.Must(messageConsumers.Commit(ctx, tp.Id, groupId, claim2.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures))
	committed = advance(ctx, cursorAdvancerDatastore, tp.Id)
	fmt.Printf("  claimed (%d,%d], committed after roller tick = %d\n", claim2.Lease.Low, claim2.Lease.High, committed)
	assertInt64("claimed moved past the pin", claimedCol(ctx, ds, tp.Id), claim2.Lease.High)
	assertInt64("committed still pinned on the unresolved exception", committedCol(ctx, ds, tp.Id), failingId-1)
	fmt.Println("  -> an unresolved exception never blocks fresh ranges from claiming/committing, only committed")

	// Commit's exception write always sets an initial 5s can_run_after -- the exception isn't
	// claimable until that backoff passes, same as reclaim's lease-expiry wait.
	step("sleep 5.5s — let the unresolved exception's initial backoff pass")
	time.Sleep(5500 * time.Millisecond)

	// ===== drain the exception window: message 3 retried and succeeds =====
	step("ClaimExceptions drains message 3, retry succeeds")
	claimedExceptions, err := exceptionConsumers.Claim(ctx, tp.Id, groupId, 1, batch, 3, lease, tp.DeliveryLogMode)
	common.Must(err)
	if len(claimedExceptions) != 1 || claimedExceptions[0].MessageId != failingId {
		common.Die(fmt.Sprintf("expected to claim exactly message %d, got %+v", failingId, claimedExceptions))
	}
	fmt.Printf("  claimed exception message_id=%d attempts=%d\n", claimedExceptions[0].MessageId, claimedExceptions[0].Attempts)
	common.Must(exceptionConsumers.RecordSuccess(ctx, &claimedExceptions[0], tp.DeliveryLogMode, nil))
	assertInt64("exception pop-deleted on success", deliveries(ctx, ds, tp.Id), 0)

	// ===== committed jumps straight past the resolved exception =====
	step("roller tick — committed jumps past the resolved exception")
	committed = advance(ctx, cursorAdvancerDatastore, tp.Id)
	fmt.Printf("  committed = %d\n", committed)
	assertInt64("committed jumped to claimed", committedCol(ctx, ds, tp.Id), claimedCol(ctx, ds, tp.Id))

	// ===== drain the rest so committed reaches head =====
	step("drain remaining ranges -> committed reaches head")
	for range 10 {
		c, err := messageConsumers.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, batch, maxRangeReclaims, lease, stream.DeliveryLogModeFailures)
		common.Must(err)
		if c == nil {
			break // caught up
		}
		common.Must(messageConsumers.Commit(ctx, tp.Id, groupId, c.Lease.Token, nil, 5*time.Second, stream.DeliveryLogModeFailures))
		fmt.Printf("  drained (%d,%d] -> committed = %d\n", c.Lease.Low, c.Lease.High, advance(ctx, cursorAdvancerDatastore, tp.Id))
	}
	assertInt64("committed reached head", committedCol(ctx, ds, tp.Id), head)
	assertInt64("no deliveries left behind", deliveries(ctx, ds, tp.Id), 0)

	fmt.Println("\n✅ PHASE 6.5c E2E TEST PASSED")
	fmt.Println("   failure recorded as an unresolved exception -> committed pinned below it while later ranges")
	fmt.Println("   kept committing -> exception resolved -> committed jumped straight past it.")
	return nil
}

// ---- helpers ----

func advance(ctx context.Context, cursorAdvancerDatastore *cursoradvancerdatastore.CursorAdvancerDatastore, streamId int64) int64 {
	c, err := cursorAdvancerDatastore.AdvanceCommitted(ctx, streamId, groupId)
	common.Must(err)
	return c
}

func committedCol(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) int64 {
	return scalar(ctx, ds, fmt.Sprintf(`SELECT committed FROM %s.%s WHERE consumer_group_id=$1`, ds.Schema, stream.ConsumerGroupCursorTable(streamId)), groupId)
}
func claimedCol(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) int64 {
	return scalar(ctx, ds, fmt.Sprintf(`SELECT claimed FROM %s.%s WHERE consumer_group_id=$1`, ds.Schema, stream.ConsumerGroupCursorTable(streamId)), groupId)
}
func deliveries(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) int64 {
	return scalar(ctx, ds, fmt.Sprintf(`SELECT count(*) FROM %s.%s WHERE consumer_group_id=$1`, ds.Schema, stream.ExceptionQueueTable(streamId)), groupId)
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

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
func assertInt64(label string, got, want int64) {
	if got != want {
		common.Die(fmt.Sprintf("%s: got %d, want %d", label, got, want))
	}
	fmt.Printf("  ✓ %s (%d)\n", label, got)
}

func mustGroupID(g *consume.Consumer, err error) int64 { common.Must(err); return g.Id }
