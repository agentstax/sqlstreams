// Command exclusive proves the dispatch-time concurrency policy on the cursor
// path: ConcurrencyExclusive messages run under an exclusive key lease, resolve
// deferred while the key is busy (the range commit writes each one's
// 'deferred' delivery row), and resolve superseded when a newer message on
// the key exists.
//
// Registers its own stream, self-seeds keyed messages, fully self-contained.
//
// Confirms, in order:
//   - a Exclusive message on a free key runs while HOLDING the key lease and
//     releases it on success -- no delivery or delivery_log rows.
//   - an Parallel (unset policy) keyed message runs with zero key lease rows.
//   - an unkeyed message under ConcurrencyOverride Exclusive runs as Parallel.
//   - ConcurrencyOverride Parallel beats a message's own Exclusive.
//   - key busy -> every head deferred during the hold ends with its own
//     'deferred' delivery row and 'deferred' log row once its range commits;
//     the rows sit inert; the key frees after the holder finishes.
//   - a message claimed as head but no longer head at dispatch resolves
//     superseded: never runs, delivery_log status 'superseded', no delivery row.
//   - a failing Exclusive message still frees the key.
//   - redemption: a stale 'deferred' row resolves superseded with its log row
//     and never runs; the head's row runs and pops.
//   - a row whose message key has an unexpired lease is never claimed:
//     no attempts motion, no log rows; the kill backstop never touches a
//     'deferred' row; the run lands once the key frees.
//   - a failed holder's retry passes the key gate and resolves superseded once
//     a newer head exists, its claim-time attempts increment decremented back.
//   - a crashed holder's expired key lease: redemption takes the key over.
//   - Concurrency Exclusive without a MessageKey is refused at produce time.
//   - uncompacted exclusive (a key, no Compaction): no compaction_head row is
//     written, every version runs exactly once serialized on the key, and a
//     same-batch key collision re-defers the loser instead of superseding it.
//   - torture: two cursor consumers and two exception consumers fight one key
//     through an abandoned (past-Timeout) holder and head churn -- only the
//     final head ever runs, exactly once, every other version audits out
//     'superseded'.
//   - destroying the stream drops the exception queue cleanly.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iCommon "github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	"github.com/agentstax/sqlstreams/pkg/consume/exceptionconsumer"
	exceptionconsumercontroller "github.com/agentstax/sqlstreams/pkg/consume/exceptionconsumer/controller"
	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer"
	"github.com/agentstax/sqlstreams/pkg/consumer"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	metricsproducer "github.com/agentstax/sqlstreams/pkg/metric/producer"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
)

type Rec struct {
	Key     string `json:"key"`
	Version int    `json:"version"`
}

func (Rec) SchemaVersion() int { return 1 }

var (
	ds       *iDatastore.PostgresDatastore
	streamId int64

	runsMu sync.Mutex
	runs   = map[string]int{} // "key:version" -> completed consumerFunc calls

	// a background start* helper's unexpected Run error lands here, so waitFor
	// reports the error instead of timing out on a consumer that already died
	backgroundRunErrors = make(chan error, 8)
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

	pool, err := common.NewPool(ctx, nil)
	common.Must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err = iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	streamName := fmt.Sprintf("exclusive.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{})
	common.Must(err)
	streamId = tp.Id

	cd, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)
	exceptionConsumers, err := exceptionconsumercontroller.NewExceptionConsumerGroupController(ds, ds.Logger)
	common.Must(err)
	wpInstance, err := client.Stream[Rec](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)

	step("exclusive on a free key: runs holding the key lease, releases on success")
	publish(ctx, wpInstance, "u:1", 1, iCommon.ConcurrencyExclusive)
	g1 := groupId(ctx, cd, "exclusive.g1")
	var heldDuringRun int
	consumeGroup(ctx, tp.Name, "exclusive.g1", nil, 3, func(ctx context.Context, message *Rec) error {
		if message.Key == "u:1" {
			heldDuringRun = leaseCount(ctx, g1)
		}
		record(message)
		return nil
	}, func() bool { return ran("u:1", 1) })
	if heldDuringRun != 1 {
		common.Die(fmt.Sprintf("want the key lease held during the run, count=%d", heldDuringRun))
	}
	if n := leaseCount(ctx, g1); n != 0 {
		common.Die(fmt.Sprintf("want the key released after success, count=%d", n))
	}
	if n := deliveryCount(ctx, g1, ""); n != 0 {
		common.Die(fmt.Sprintf("a clean Exclusive run must leave no delivery rows, got %d", n))
	}
	fmt.Println("  ✓ held during, released after, no rows")

	step("parallel (unset policy) keyed message never touches the key lease")
	publish(ctx, wpInstance, "u:2", 1, "")
	g2 := groupId(ctx, cd, "exclusive.g2")
	allowHeld := -1
	consumeGroup(ctx, tp.Name, "exclusive.g2", nil, 3, func(ctx context.Context, message *Rec) error {
		if message.Key == "u:2" {
			allowHeld = leaseCount(ctx, g2)
		}
		record(message)
		return nil
	}, func() bool { return ran("u:2", 1) })
	if allowHeld != 0 {
		common.Die(fmt.Sprintf("an Parallel run must not hold a key lease, count=%d", allowHeld))
	}
	fmt.Println("  ✓ no lease rows")

	step("unkeyed under ConcurrencyOverride Exclusive runs as Parallel")
	publishUnkeyed(ctx, wpInstance, 1)
	g3 := groupId(ctx, cd, "exclusive.g3")
	unkeyedHeld := -1
	consumeGroup(ctx, tp.Name, "exclusive.g3", &messageconsumer.MessageConsumerConfig{ConcurrencyOverride: iCommon.ConcurrencyExclusive}, 3, func(ctx context.Context, message *Rec) error {
		if message.Key == "" {
			unkeyedHeld = leaseCount(ctx, g3)
		}
		record(message)
		return nil
	}, func() bool { return ran("", 1) })
	if unkeyedHeld != 0 {
		common.Die(fmt.Sprintf("an unkeyed run must not hold a key lease even under override Exclusive, count=%d", unkeyedHeld))
	}
	fmt.Println("  ✓ no lease rows")

	step("ConcurrencyOverride Parallel beats a message's own Exclusive")
	publish(ctx, wpInstance, "u:3", 1, iCommon.ConcurrencyExclusive)
	g4 := groupId(ctx, cd, "exclusive.g4")
	overrideHeld := -1
	consumeGroup(ctx, tp.Name, "exclusive.g4", &messageconsumer.MessageConsumerConfig{ConcurrencyOverride: iCommon.ConcurrencyParallel}, 3, func(ctx context.Context, message *Rec) error {
		if message.Key == "u:3" {
			overrideHeld = leaseCount(ctx, g4)
		}
		record(message)
		return nil
	}, func() bool { return ran("u:3", 1) })
	if overrideHeld != 0 {
		common.Die(fmt.Sprintf("override Parallel must skip the key lease, count=%d", overrideHeld))
	}
	fmt.Println("  ✓ ran without a lease")

	step("busy key: each head deferred during the hold gets its own 'deferred' row at commit")
	g5 := groupId(ctx, cd, "exclusive.g5")
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	publish(ctx, wpInstance, "u:4", 1, iCommon.ConcurrencyExclusive)
	v1 := messageId(ctx, "u:4", 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		consumeGroup(ctx, tp.Name, "exclusive.g5", nil, 3, func(ctx context.Context, message *Rec) error {
			if message.Key == "u:4" {
				startOnce.Do(func() { close(started) })
				<-release
			}
			record(message)
			return nil
		}, func() bool { return ran("u:4", 1) })
	}()

	<-started // v1 is running and holds the key
	publish(ctx, wpInstance, "u:4", 2, iCommon.ConcurrencyExclusive)
	v2 := messageId(ctx, "u:4", 2)
	waitFor(func() bool { return deliveryStatus(ctx, g5, v2) == "deferred" }, "v2's 'deferred' row")

	publish(ctx, wpInstance, "u:4", 3, iCommon.ConcurrencyExclusive)
	v3 := messageId(ctx, "u:4", 3)
	waitFor(func() bool { return deliveryStatus(ctx, g5, v3) == "deferred" }, "v3's 'deferred' row")
	if s := deliveryStatus(ctx, g5, v2); s != "deferred" {
		common.Die(fmt.Sprintf("v2's 'deferred' row must sit untouched next to v3's, got status %q", s))
	}
	if n := deliveryCount(ctx, g5, "deferred"); n != 2 {
		common.Die(fmt.Sprintf("want a 'deferred' row for each head deferred during the hold, got %d", n))
	}
	for _, v := range []int64{v2, v3} {
		if n := logCount(ctx, g5, v); n != 1 {
			common.Die(fmt.Sprintf("want one 'deferred' log row for message %d, got %d", v, n))
		}
		if s, _ := logRow(ctx, g5, v); s != "deferred" {
			common.Die(fmt.Sprintf("message %d's log row status = %q, want deferred", v, s))
		}
	}

	close(release)
	<-done // v1 finished
	if n := leaseCount(ctx, g5); n != 0 {
		common.Die(fmt.Sprintf("want the key released after the holder finished, count=%d", n))
	}
	if ran("u:4", 2) || ran("u:4", 3) {
		common.Die("deferred messages must stay inert until an ExceptionConsumer runs")
	}
	if deliveryStatus(ctx, g5, v2) != "deferred" || deliveryStatus(ctx, g5, v3) != "deferred" {
		common.Die("both 'deferred' rows must survive the holder's release untouched")
	}
	fmt.Printf("  ✓ v2 and v3 each hold a 'deferred' row, key freed (v1=%d v2=%d v3=%d)\n", v1, v2, v3)

	step("head moved between claim and dispatch: resolves superseded, never runs")
	g6 := groupId(ctx, cd, "exclusive.g6")
	blockStarted := make(chan struct{})
	blockRelease := make(chan struct{})
	var blockOnce sync.Once
	publishUnkeyed(ctx, wpInstance, 2) // the blocker -- pins the single processor
	publish(ctx, wpInstance, "u:5", 1, iCommon.ConcurrencyExclusive)
	v5old := messageId(ctx, "u:5", 1)

	done6 := make(chan struct{})
	go func() {
		defer close(done6)
		consumeGroup(ctx, tp.Name, "exclusive.g6", nil, 1, func(ctx context.Context, message *Rec) error {
			if message.Key == "" && message.Version == 2 {
				blockOnce.Do(func() { close(blockStarted) })
				<-blockRelease
			}
			record(message)
			return nil
		}, func() bool { return ran("u:5", 2) })
	}()

	<-blockStarted // u:5 v1 is claimed and queued behind the blocker
	publish(ctx, wpInstance, "u:5", 2, iCommon.ConcurrencyExclusive)
	close(blockRelease)
	<-done6 // v2 ran -- so v1 resolved before it
	if ran("u:5", 1) {
		common.Die("the stale claimed message ran -- dispatch must re-check the head")
	}
	status6, _ := logRow(ctx, g6, v5old)
	if status6 != "superseded" {
		common.Die(fmt.Sprintf("stale message's log row status = %q, want superseded", status6))
	}
	if s := deliveryStatus(ctx, g6, v5old); s != "" {
		common.Die(fmt.Sprintf("a dispatch-superseded message must not enter the delivery window, got status %q", s))
	}
	fmt.Println("  ✓ never ran, logged superseded, no delivery row")

	step("a failing Exclusive message frees the key")
	g7 := groupId(ctx, cd, "exclusive.g7")
	publish(ctx, wpInstance, "u:6", 1, iCommon.ConcurrencyExclusive)
	v6 := messageId(ctx, "u:6", 1)
	consumeGroup(ctx, tp.Name, "exclusive.g7", nil, 3, func(ctx context.Context, message *Rec) error {
		record(message)
		if message.Key == "u:6" {
			return errors.New("exclusive: synthetic failure")
		}
		return nil
	}, func() bool { return deliveryStatus(ctx, g7, v6) == "ready" })
	if n := leaseCount(ctx, g7); n != 0 {
		common.Die(fmt.Sprintf("want the key released after the exception, count=%d", n))
	}
	status7, _ := logRow(ctx, g7, v6)
	if status7 != "failure" {
		common.Die(fmt.Sprintf("exception log row status = %q, want failure", status7))
	}
	fmt.Println("  ✓ key freed, 'ready' row, status failure")

	step("redemption: the stale 'deferred' row resolves superseded, the head runs and pops")
	stopRedeem := startExceptionConsumer(ctx, tp.Name, "exclusive.g5", nil, func(ctx context.Context, message *Rec) error {
		record(message)
		return nil
	})
	// ran() flips inside consumerFunc, before the outcome lands -- wait on the
	// recorded state, not the run
	waitFor(func() bool {
		return ran("u:4", 3) && deliveryStatus(ctx, g5, v2) == "superseded" && deliveryStatus(ctx, g5, v3) == ""
	}, "v3 to run and pop, v2 to resolve superseded")
	stopRedeem()
	if ran("u:4", 2) {
		common.Die("a stale 'deferred' message must never run")
	}
	if n := deliveryAttempts(ctx, g5, v2); n != 0 {
		common.Die(fmt.Sprintf("a superseded 'deferred' row never ran, attempts must net 0, got %d", n))
	}
	// v2's audit trail: 'deferred' at attempt 0 from its range commit,
	// 'superseded' at attempt 1 from redemption
	statuses := logStatuses(ctx, g5, v2)
	if len(statuses) != 2 || statuses[0] != "deferred" || statuses[1] != "superseded" {
		common.Die(fmt.Sprintf("v2's log rows = %v, want deferred at 0 and superseded at 1", statuses))
	}
	if n := logCount(ctx, g5, v3); n != 1 {
		common.Die(fmt.Sprintf("a redeemed success leaves only its commit-time 'deferred' log row, got %d rows", n))
	}
	if n := leaseCount(ctx, g5); n != 0 {
		common.Die(fmt.Sprintf("want the key released after redemption, count=%d", n))
	}
	fmt.Println("  ✓ v2 superseded with full audit, v3 ran and popped")

	step("a held key excludes its 'deferred' row from the claim, kill backstop blind to it")
	g8 := groupId(ctx, cd, "exclusive.g8")
	started8 := make(chan struct{})
	release8 := make(chan struct{})
	var once8 sync.Once
	stopCursor8 := startConsumer(ctx, tp.Name, "exclusive.g8", nil, 3, func(ctx context.Context, message *Rec) error {
		if message.Key == "u:7" && message.Version == 1 {
			once8.Do(func() { close(started8) })
			<-release8
		}
		record(message)
		return nil
	})
	publish(ctx, wpInstance, "u:7", 1, iCommon.ConcurrencyExclusive)
	<-started8 // v1 is running and holds the key
	publish(ctx, wpInstance, "u:7", 2, iCommon.ConcurrencyExclusive)
	v7 := messageId(ctx, "u:7", 2)
	waitFor(func() bool { return deliveryStatus(ctx, g8, v7) == "deferred" }, "v2's 'deferred' row")

	// exhausted-looking or not, a 'deferred' row is outside the kill
	// backstop's 'inflight' predicate. Driven directly so no consumer touches
	// the row mid-check.
	execSql(ctx, fmt.Sprintf(`UPDATE %s.%s SET attempts = 99, lease_expires_at = now() - interval '1 minute' WHERE consumer_group_id = $1 AND message_id = $2`, ds.Schema, stream.ExceptionQueueTable(streamId)), g8, v7)
	if _, err := exceptionConsumers.Kill(ctx, tp.Id, g8, 3, stream.DeliveryLogModeFailures); err != nil {
		common.Die(fmt.Sprintf("Kill: %v", err))
	}
	if s := deliveryStatus(ctx, g8, v7); s != "deferred" {
		common.Die(fmt.Sprintf("the kill backstop must never touch a 'deferred' row, got status %q", s))
	}
	if _, err := exceptionConsumers.Claim(ctx, tp.Id, g8, 1, 10, 3, 5*time.Second, stream.DeliveryLogModeFailures); err != nil {
		common.Die(fmt.Sprintf("ClaimExceptions: %v", err))
	}
	if n := deliveryAttempts(ctx, g8, v7); n != 99 {
		common.Die(fmt.Sprintf("an exhausted row must not be claimed, attempts = %d", n))
	}
	// the unexpired message_key_lease row alone must exclude the row -- attempts back at
	// 0, well under the ceiling
	execSql(ctx, fmt.Sprintf(`UPDATE %s.%s SET attempts = 0 WHERE consumer_group_id = $1 AND message_id = $2`, ds.Schema, stream.ExceptionQueueTable(streamId)), g8, v7)
	if _, err := exceptionConsumers.Claim(ctx, tp.Id, g8, 1, 10, 3, 5*time.Second, stream.DeliveryLogModeFailures); err != nil {
		common.Die(fmt.Sprintf("ClaimExceptions: %v", err))
	}
	if s, n := deliveryStatus(ctx, g8, v7), deliveryAttempts(ctx, g8, v7); s != "deferred" || n != 0 {
		common.Die(fmt.Sprintf("a row whose message key has an unexpired lease must not be claimed, got status %q attempts %d", s, n))
	}

	stopRedeem8 := startExceptionConsumer(ctx, tp.Name, "exclusive.g8", nil, func(ctx context.Context, message *Rec) error {
		record(message)
		return nil
	})
	time.Sleep(300 * time.Millisecond) // several claim polls against the held key
	if ran("u:7", 2) {
		common.Die("nothing may run while another delivery holds the key")
	}
	// the row is never claimed while the key is held -- status and attempts
	// hold still, so read them directly, no sampling
	if s, n := deliveryStatus(ctx, g8, v7), deliveryAttempts(ctx, g8, v7); s != "deferred" || n != 0 {
		common.Die(fmt.Sprintf("live claim polls must leave the excluded row untouched, got status %q attempts %d", s, n))
	}
	if n := logCount(ctx, g8, v7); n != 1 {
		common.Die(fmt.Sprintf("an unclaimed row must not grow log rows, got %d", n))
	}

	close(release8)
	waitFor(func() bool { return ran("u:7", 2) }, "v2 to run once the key freed")
	waitFor(func() bool { return deliveryStatus(ctx, g8, v7) == "" && leaseCount(ctx, g8) == 0 }, "v2's row to pop and the key to free")
	stopCursor8()
	stopRedeem8()
	fmt.Println("  ✓ excluded from the claim while held, survived the backstop, ran on release")

	step("a failed holder's retry supersedes once a newer head runs")
	g9 := groupId(ctx, cd, "exclusive.g9")
	failV1 := func(ctx context.Context, message *Rec) error {
		record(message)
		if message.Key == "u:8" && message.Version == 1 {
			return errors.New("exclusive: synthetic holder failure")
		}
		return nil
	}
	stopCursor9 := startConsumer(ctx, tp.Name, "exclusive.g9", nil, 3, failV1)
	stopRedeem9 := startExceptionConsumer(ctx, tp.Name, "exclusive.g9", nil, failV1)
	publish(ctx, wpInstance, "u:8", 1, iCommon.ConcurrencyExclusive)
	v8old := messageId(ctx, "u:8", 1)
	waitFor(func() bool { return deliveryStatus(ctx, g9, v8old) == "ready" }, "v1 to fail and go 'ready'")
	publish(ctx, wpInstance, "u:8", 2, iCommon.ConcurrencyExclusive)
	waitFor(func() bool { return ran("u:8", 2) }, "v2 to run on the freed key")
	waitFor(func() bool { return deliveryStatus(ctx, g9, v8old) == "superseded" }, "v1's retry to resolve superseded")
	stopCursor9()
	stopRedeem9()
	// the superseded log row lands one above the last counted attempt --
	// RecordExceptionSuperseded decremented the refused claim's increment back
	statuses9 := logStatuses(ctx, g9, v8old)
	sup := -1
	for attempt, s := range statuses9 {
		if s == "superseded" {
			sup = attempt
		}
	}
	if sup == -1 {
		common.Die(fmt.Sprintf("v1's log rows = %v, want a superseded row", statuses9))
	}
	if att := deliveryAttempts(ctx, g9, v8old); sup != att+1 {
		common.Die(fmt.Sprintf("superseded logged at attempt %d with attempts %d, want attempts + 1", sup, att))
	}
	if n := leaseCount(ctx, g9); n != 0 {
		common.Die(fmt.Sprintf("want the key released, count=%d", n))
	}
	fmt.Println("  ✓ retry gate refused v1, attempts decremented back, superseded logged")

	step("a crashed holder's expired key lease: redemption takes the key over")
	g10 := groupId(ctx, cd, "exclusive.g10")
	// a crashed holder's message_key_lease row: unexpired, never released
	execSql(ctx, fmt.Sprintf(`INSERT INTO %s.%s (consumer_group_id, message_key, token, expires_at) VALUES ($1, 'u:10', gen_random_uuid(), now() + interval '1500 milliseconds')`, ds.Schema, stream.MessageKeyLeaseTable(streamId)), g10)
	publish(ctx, wpInstance, "u:10", 1, iCommon.ConcurrencyExclusive)
	v10 := messageId(ctx, "u:10", 1)
	stopCursor10 := startConsumer(ctx, tp.Name, "exclusive.g10", nil, 3, func(ctx context.Context, message *Rec) error {
		record(message)
		return nil
	})
	stopRedeem10 := startExceptionConsumer(ctx, tp.Name, "exclusive.g10", nil, func(ctx context.Context, message *Rec) error {
		record(message)
		return nil
	})
	waitFor(func() bool { return deliveryStatus(ctx, g10, v10) == "deferred" }, "v1 to wait behind the crashed holder's key")
	if ran("u:10", 1) {
		common.Die("nothing may run while the crashed holder's lease is live")
	}
	waitFor(func() bool { return ran("u:10", 1) }, "redemption to take the expired key over")
	waitFor(func() bool { return deliveryStatus(ctx, g10, v10) == "" && leaseCount(ctx, g10) == 0 }, "the row to pop and the key to free")
	stopCursor10()
	stopRedeem10()
	fmt.Println("  ✓ deferred behind the crashed holder, ran after expiry via takeover")

	step("Exclusive without a MessageKey is refused at produce time")
	if _, err := wpInstance.Produce(ctx, &Rec{Version: 1}, &sqlstreams.ProduceOptions{Message: &iCommon.MessageOptions{Concurrency: iCommon.ConcurrencyExclusive}}); err == nil {
		common.Die("produce must refuse Exclusive without a MessageKey")
	}
	fmt.Println("  ✓ refused")

	step("uncompacted exclusive: no head row, every version runs exactly once, none superseded")
	g12 := groupId(ctx, cd, "exclusive.g12")
	started12 := make(chan struct{})
	release12 := make(chan struct{})
	var once12 sync.Once
	publishUncompacted(ctx, wpInstance, "uc:1", 1)
	uc1 := messageId(ctx, "uc:1", 1)

	done12 := make(chan struct{})
	go func() {
		defer close(done12)
		consumeGroup(ctx, tp.Name, "exclusive.g12", nil, 3, func(ctx context.Context, message *Rec) error {
			if message.Key == "uc:1" && message.Version == 1 {
				once12.Do(func() { close(started12) })
				<-release12
			}
			record(message)
			return nil
		}, func() bool { return ran("uc:1", 1) })
	}()

	<-started12 // v1 is running and holds the key
	publishUncompacted(ctx, wpInstance, "uc:1", 2)
	uc2 := messageId(ctx, "uc:1", 2)
	waitFor(func() bool { return deliveryStatus(ctx, g12, uc2) == "deferred" }, "v2's 'deferred' row")
	publishUncompacted(ctx, wpInstance, "uc:1", 3)
	uc3 := messageId(ctx, "uc:1", 3)
	waitFor(func() bool { return deliveryStatus(ctx, g12, uc3) == "deferred" }, "v3's 'deferred' row")

	var headRows12 int
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE compaction_key = 'uc:1';`, ds.Schema, stream.CompactionHeadTable(tp.Id))).Scan(&headRows12))
	if headRows12 != 0 {
		common.Die(fmt.Sprintf("an uncompacted produce must not write a compaction head, got %d rows", headRows12))
	}

	close(release12)
	<-done12 // v1 finished

	// redemption runs BOTH deferred versions -- an uncompacted key keeps its
	// history; a same-batch key collision re-defers instead of superseding
	stopRedeem12 := startExceptionConsumer(ctx, tp.Name, "exclusive.g12", nil, func(ctx context.Context, message *Rec) error {
		record(message)
		return nil
	})
	waitFor(func() bool { return ran("uc:1", 2) && ran("uc:1", 3) }, "both deferred versions to run")
	waitFor(func() bool {
		return deliveryStatus(ctx, g12, uc2) == "" && deliveryStatus(ctx, g12, uc3) == "" && leaseCount(ctx, g12) == 0
	}, "both rows to pop and the key to free")
	stopRedeem12()

	for version, id := range map[int]int64{1: uc1, 2: uc2, 3: uc3} {
		if n := runCount("uc:1", version); n != 1 {
			common.Die(fmt.Sprintf("version %d must run exactly once, ran %d times", version, n))
		}
		for _, status := range logStatuses(ctx, g12, id) {
			if status == "superseded" {
				common.Die(fmt.Sprintf("version %d resolved superseded -- an uncompacted key must keep every version", version))
			}
		}
	}
	fmt.Println("  ✓ v1 ran holding the key, v2 and v3 both redeemed, nothing superseded")

	step("torture: two consumers per loop fight one key through an abandoned holder and head churn")
	g11 := groupId(ctx, cd, "exclusive.g11")
	// a short per-message Timeout so v1's sleeping run is abandoned mid-hold --
	// its failure recording frees the key while the goroutine sleeps on
	tortureMessageCfg := func() *messageconsumer.MessageConsumerConfig {
		return &messageconsumer.MessageConsumerConfig{Message: &iCommon.MessageOptions{Timeout: 500 * time.Millisecond}}
	}
	tortureExceptionCfg := func() *exceptionconsumer.ExceptionConsumerConfig {
		return &exceptionconsumer.ExceptionConsumerConfig{Message: &iCommon.MessageOptions{Timeout: 500 * time.Millisecond}}
	}
	started11 := make(chan struct{})
	var once11 sync.Once
	tortureFunc := func(ctx context.Context, message *Rec) error {
		record(message)
		if message.Key == "u:11" && message.Version == 1 {
			once11.Do(func() { close(started11) })
			time.Sleep(2 * time.Second) // ignores ctx -- abandoned at Timeout
		}
		return nil
	}
	stopCursor11a := startConsumer(ctx, tp.Name, "exclusive.g11", tortureMessageCfg(), 3, tortureFunc)
	stopCursor11b := startConsumer(ctx, tp.Name, "exclusive.g11", tortureMessageCfg(), 3, tortureFunc)
	stopRedeem11a := startExceptionConsumer(ctx, tp.Name, "exclusive.g11", tortureExceptionCfg(), tortureFunc)
	stopRedeem11b := startExceptionConsumer(ctx, tp.Name, "exclusive.g11", tortureExceptionCfg(), tortureFunc)

	publish(ctx, wpInstance, "u:11", 1, iCommon.ConcurrencyExclusive)
	tv1 := messageId(ctx, "u:11", 1)
	<-started11 // v1 runs holding the key
	publish(ctx, wpInstance, "u:11", 2, iCommon.ConcurrencyExclusive)
	publish(ctx, wpInstance, "u:11", 3, iCommon.ConcurrencyExclusive)
	publish(ctx, wpInstance, "u:11", 4, iCommon.ConcurrencyExclusive)
	tv2 := messageId(ctx, "u:11", 2)
	tv3 := messageId(ctx, "u:11", 3)
	tv4 := messageId(ctx, "u:11", 4)

	// interleaving-robust: whichever versions deferred vs superseded at
	// dispatch, the end state is fixed -- v4 runs and pops, everything else
	// resolves without running, nothing is left unresolved
	waitFor(func() bool {
		return runCount("u:11", 4) == 1 &&
			deliveryStatus(ctx, g11, tv4) == "" &&
			deliveryStatus(ctx, g11, tv1) == "superseded" &&
			deliveryCount(ctx, g11, "ready") == 0 &&
			deliveryCount(ctx, g11, "inflight") == 0 &&
			deliveryCount(ctx, g11, "deferred") == 0 &&
			leaseCount(ctx, g11) == 0
	}, "v4 to run once, v1's retry to resolve superseded, every row to resolve")
	stopCursor11a()
	stopCursor11b()
	stopRedeem11a()
	stopRedeem11b()

	if n := runCount("u:11", 1); n != 1 {
		common.Die(fmt.Sprintf("the abandoned holder must have run exactly once, ran %d times", n))
	}
	if n := runCount("u:11", 4); n != 1 {
		common.Die(fmt.Sprintf("the final head must run exactly once across four racing consumers, ran %d times", n))
	}
	// a non-head ends one of three ways: claim-time compacted (no rows at
	// all -- the documented silent drop), dispatch-superseded (log row only),
	// or 'deferred' then redemption-superseded (delivery row + log row)
	for version, id := range map[int]int64{2: tv2, 3: tv3} {
		if ran("u:11", version) {
			common.Die(fmt.Sprintf("non-head v%d must never run", version))
		}
		status := deliveryStatus(ctx, g11, id)
		if status != "superseded" && status != "" {
			common.Die(fmt.Sprintf("v%d must end superseded or dropped, got status %q", version, status))
		}
		sup := 0
		for _, s := range logStatuses(ctx, g11, id) {
			if s == "superseded" {
				sup++
			}
		}
		if sup > 1 {
			common.Die(fmt.Sprintf("v%d must never audit more than one 'superseded' log row, got %d", version, sup))
		}
		if status == "superseded" && sup != 1 {
			common.Die(fmt.Sprintf("v%d's 'superseded' delivery row needs its log row, got %d", version, sup))
		}
	}
	fmt.Printf("  ✓ v4 ran once, v1-v3 audited out superseded (v1=%d v2=%d v3=%d v4=%d)\n", tv1, tv2, tv3, tv4)

	step("destroying the stream drops the exception queue")
	common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	fmt.Println("  ✓ destroyed")

	fmt.Println("\n✅ EXCLUSIVE E2E TEST PASSED")
	return nil
}

// consumeGroup runs a MessageConsumer for group until done() (10s cap), with pool
// concurrent processors.
func consumeGroup(ctx context.Context, streamName, group string, cfg *messageconsumer.MessageConsumerConfig, pool int, consumerFunc consumer.ConsumerFunc[Rec], done func() bool) {
	if cfg == nil {
		cfg = &messageconsumer.MessageConsumerConfig{}
	}
	cfg.BatchLimit = 50
	cfg.ClaimPollRate = 50 * time.Millisecond

	cfg.QueueSize = 50
	cfg.MessageConcurrency = pool

	owner := groupOwner(ctx, streamName, group)
	definition, err := messageconsumer.NewMessageConsumerProvisioner(ds, consumerFunc, 1, abandonedEventProducer(ctx), cfg, ds.Logger)
	common.Must(err)

	runCtx, cancel := context.WithCancel(ctx)
	execution := claimOne(runCtx, definition, owner)
	errCh := make(chan error, 1)
	go func() { errCh <- execution.Run(runCtx) }()

	start := time.Now()
	for !done() {
		if time.Since(start) > 10*time.Second {
			cancel()
			common.Die(fmt.Sprintf("timed out waiting for %s to finish, Run returned: %v", group, <-errCh))
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
		common.Die(fmt.Sprintf("Run returned an unexpected error: %v", err))
	}
}

// startConsumer runs a MessageConsumer for group until the returned stop is
// called. cfg may be nil.
func startConsumer(ctx context.Context, streamName, group string, cfg *messageconsumer.MessageConsumerConfig, pool int, consumerFunc consumer.ConsumerFunc[Rec]) func() {
	if cfg == nil {
		cfg = &messageconsumer.MessageConsumerConfig{}
	}
	cfg.BatchLimit = 50
	cfg.ClaimPollRate = 50 * time.Millisecond
	cfg.QueueSize = 50
	cfg.MessageConcurrency = pool

	owner := groupOwner(ctx, streamName, group)
	definition, err := messageconsumer.NewMessageConsumerProvisioner(ds, consumerFunc, 1, abandonedEventProducer(ctx), cfg, ds.Logger)
	common.Must(err)

	runCtx, cancel := context.WithCancel(ctx)
	execution := claimOne(runCtx, definition, owner)
	errCh := make(chan error, 1)
	go func() {
		err := execution.Run(runCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			backgroundRunErrors <- err
		}
		errCh <- err
	}()
	return func() {
		cancel()
		if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
			common.Die(fmt.Sprintf("Run returned an unexpected error: %v", err))
		}
	}
}

// startExceptionConsumer runs an ExceptionConsumer (exception retries +
// deferred redemption, one claim) for group until the returned stop is
// called. cfg may be nil.
func startExceptionConsumer(ctx context.Context, streamName, group string, cfg *exceptionconsumer.ExceptionConsumerConfig, consumerFunc consumer.ConsumerFunc[Rec]) func() {
	if cfg == nil {
		cfg = &exceptionconsumer.ExceptionConsumerConfig{}
	}
	cfg.BatchLimit = 50
	cfg.ClaimPollRate = 50 * time.Millisecond

	owner := groupOwner(ctx, streamName, group)
	definition, err := exceptionconsumer.NewExceptionConsumerProvisioner(ds, consumerFunc, 1, abandonedEventProducer(ctx), cfg, ds.Logger)
	common.Must(err)

	runCtx, cancel := context.WithCancel(ctx)
	execution := claimOne(runCtx, definition, owner)
	errCh := make(chan error, 1)
	go func() {
		err := execution.Run(runCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			backgroundRunErrors <- err
		}
		errCh <- err
	}()
	return func() {
		cancel()
		if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
			common.Die(fmt.Sprintf("exception Run returned an unexpected error: %v", err))
		}
	}
}

func groupOwner(ctx context.Context, streamName string, group string) *iCommon.Owner {
	streamController, err := streamcontroller.NewStreamController(ds, ds.Logger)
	common.Must(err)
	tp, err := streamController.Get(ctx, streamName)
	common.Must(err)

	consumerDatastore, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)
	g, err := consumerDatastore.RegisterGroup(ctx, tp.Id, group, consume.Beginning())
	common.Must(err)

	owner, err := iCommon.NewConsumerGroupOwner(tp.SystemId, tp.Id, g.Id, g.Name)
	common.Must(err)
	return owner
}

func abandonedEventProducer(ctx context.Context) *metricsproducer.MetricProducer {
	events, err := metricsproducer.NewMetricsProducer(ds, nil, ds.Logger)
	common.Must(err)
	go func() {
		common.Must(events.Run(ctx, "exclusive", "exclusive", 1, "exclusive-session"))
	}()
	return events
}

// no manager, so nothing respawns the execution -- the e2e test decides exactly how
// many run
func claimOne(ctx context.Context, provisioner declaringProvisioner, owner *iCommon.Owner) worker.Execution {
	common.Must(provisioner.Declare(ctx, owner))

	workers, err := workercontroller.NewWorkerController(ds, ds.Logger)
	common.Must(err)
	row, err := workers.GetWorker(ctx, provisioner.Definition().Name, owner)
	common.Must(err)

	execution, err := provisioner.Provision(ctx, row)
	common.Must(err)
	return execution
}

// every consumer provisioner both declares its row and provisions it
type declaringProvisioner interface {
	worker.Declarer
	worker.Provisioner
}

func record(message *Rec) {
	runsMu.Lock()
	defer runsMu.Unlock()
	runs[fmt.Sprintf("%s:%d", message.Key, message.Version)]++
}

func runCount(key string, version int) int {
	runsMu.Lock()
	defer runsMu.Unlock()
	return runs[fmt.Sprintf("%s:%d", key, version)]
}

func ran(key string, version int) bool {
	runsMu.Lock()
	defer runsMu.Unlock()
	return runs[fmt.Sprintf("%s:%d", key, version)] > 0
}

func publish(ctx context.Context, wpInstance *sqlstreams.ProducerInstance[Rec], key string, version int, policy iCommon.ConcurrencyPolicy) {
	opts := &sqlstreams.ProduceOptions{MessageKey: key, Compaction: &sqlstreams.CompactionOptions{Enable: true}}
	if policy != "" {
		opts.Message = &iCommon.MessageOptions{Concurrency: policy}
	}
	_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*Rec, error) {
		return &Rec{Key: key, Version: version}, nil
	}, opts)
	common.Must(err)
}

// publishUncompacted produces a Exclusive message with a key and no Compaction --
// every version is kept, deliveries serialize on the key.
func publishUncompacted(ctx context.Context, wpInstance *sqlstreams.ProducerInstance[Rec], key string, version int) {
	_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*Rec, error) {
		return &Rec{Key: key, Version: version}, nil
	}, &sqlstreams.ProduceOptions{
		MessageKey: key,
		Message:    &iCommon.MessageOptions{Concurrency: iCommon.ConcurrencyExclusive},
	})
	common.Must(err)
}

func publishUnkeyed(ctx context.Context, wpInstance *sqlstreams.ProducerInstance[Rec], version int) {
	_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*Rec, error) {
		return &Rec{Version: version}, nil
	}, nil)
	common.Must(err)
}

func groupId(ctx context.Context, cd *consumecontroller.ConsumeController, name string) int64 {
	g, err := cd.RegisterGroup(ctx, streamId, name, consume.Beginning())
	common.Must(err)
	return g.Id
}

func messageId(ctx context.Context, key string, version int) int64 {
	var id int64
	sql := fmt.Sprintf(`SELECT id FROM %s.%s WHERE message_key = $1 AND (payload->>'version')::int = $2`, ds.Schema, stream.MessageLogTable(streamId))
	common.Must(ds.Pool.QueryRow(ctx, sql, key, version).Scan(&id))
	return id
}

func leaseCount(ctx context.Context, groupId int64) int {
	var n int
	sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = $1`, ds.Schema, stream.MessageKeyLeaseTable(streamId))
	common.Must(ds.Pool.QueryRow(ctx, sql, groupId).Scan(&n))
	return n
}

// deliveryCount counts the group's delivery rows; status "" counts them all.
func deliveryCount(ctx context.Context, groupId int64, status string) int {
	var n int
	sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = $1 AND ($2 = '' OR status = $2)`, ds.Schema, stream.ExceptionQueueTable(streamId))
	common.Must(ds.Pool.QueryRow(ctx, sql, groupId, status).Scan(&n))
	return n
}

// deliveryStatus returns "" when the message has no delivery row.
func deliveryStatus(ctx context.Context, groupId int64, messageId int64) string {
	var s string
	sql := fmt.Sprintf(`SELECT COALESCE(MAX(status), '') FROM %s.%s WHERE consumer_group_id = $1 AND message_id = $2`, ds.Schema, stream.ExceptionQueueTable(streamId))
	common.Must(ds.Pool.QueryRow(ctx, sql, groupId, messageId).Scan(&s))
	return s
}

func deliveryAttempts(ctx context.Context, groupId int64, messageId int64) int {
	var n int
	sql := fmt.Sprintf(`SELECT attempts FROM %s.%s WHERE consumer_group_id = $1 AND message_id = $2`, ds.Schema, stream.ExceptionQueueTable(streamId))
	common.Must(ds.Pool.QueryRow(ctx, sql, groupId, messageId).Scan(&n))
	return n
}

// logStatuses returns the message's delivery_log statuses keyed by attempt.
func logStatuses(ctx context.Context, groupId int64, messageId int64) map[int]string {
	sql := fmt.Sprintf(`SELECT attempt, status FROM %s.%s WHERE consumer_group_id = $1 AND message_id = $2`, ds.Schema, stream.DeliveryLogTable(streamId))
	rows, err := ds.Pool.Query(ctx, sql, groupId, messageId)
	common.Must(err)
	defer rows.Close()

	statuses := map[int]string{}
	for rows.Next() {
		var attempt int
		var status string
		common.Must(rows.Scan(&attempt, &status))
		statuses[attempt] = status
	}
	common.Must(rows.Err())
	return statuses
}

func execSql(ctx context.Context, sql string, args ...any) {
	_, err := ds.Pool.Exec(ctx, sql, args...)
	common.Must(err)
}

func logCount(ctx context.Context, groupId int64, messageId int64) int {
	var n int
	sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = $1 AND message_id = $2`, ds.Schema, stream.DeliveryLogTable(streamId))
	common.Must(ds.Pool.QueryRow(ctx, sql, groupId, messageId).Scan(&n))
	return n
}

// logRow returns the message's single delivery_log row's status and error.
func logRow(ctx context.Context, groupId int64, messageId int64) (string, string) {
	var status, logErr string
	sql := fmt.Sprintf(`SELECT status, error FROM %s.%s WHERE consumer_group_id = $1 AND message_id = $2`, ds.Schema, stream.DeliveryLogTable(streamId))
	common.Must(ds.Pool.QueryRow(ctx, sql, groupId, messageId).Scan(&status, &logErr))
	return status, logErr
}

func waitFor(cond func() bool, what string) {
	start := time.Now()
	for !cond() {
		select {
		case err := <-backgroundRunErrors:
			common.Die(fmt.Sprintf("a background Run returned an unexpected error while waiting for %s: %v", what, err))
		default:
		}
		if time.Since(start) > 10*time.Second {
			common.Die("timed out waiting for " + what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
