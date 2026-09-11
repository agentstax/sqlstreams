// Command keylease proves the message_key_lease primitives in isolation (no
// consumer wiring yet -- dispatch integration is proven by later e2e tests).
//
// Registers its own stream, self-seeds keyed messages, fully self-contained.
//
// Confirms, in order:
//   - a stale (non-head) message resolves superseded WITHOUT creating or
//     touching a lease row -- the head check runs before the lease attempt,
//     both on a free key and while the key is held.
//   - the head message acquires when the key is free; a second attempt while
//     held is busy.
//   - a release inside a rolled-back txn leaves the lease held.
//   - a released key is immediately reacquirable.
//   - takeover only after expiry: before expires_at the key is busy, after
//     it the next claim wins with a FRESH token, and the expired holder's
//     release matches zero rows (it cannot delete the new holder's row).
//   - N concurrent acquires on one free key admit exactly one.
//   - old-then-new order while a key is held: a newer head version produced
//     mid-hold claims busy (the lease gates it, not the head), the held
//     message's own reclaim resolves superseded, and after release the new
//     head acquires.
//   - the janitor sweep removes expired rows and leaves live ones.
//   - destroying the stream drops its message_key_lease table.
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iCommon "github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	keyleasecontroller "github.com/agentstax/sqlstreams/pkg/consume/base/controller"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	janitordatastore "github.com/agentstax/sqlstreams/pkg/stream/janitor/controller/datastore"
)

const group = "keylease.group"

type Rec struct {
	Key     string `json:"key"`
	Version int    `json:"version"`
}

func (Rec) SchemaVersion() int { return 1 }

var (
	ds       *iDatastore.PostgresDatastore
	streamId int64
	groupId  int64
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

	streamName := fmt.Sprintf("keylease.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{})
	common.Must(err)
	streamId = tp.Id

	cd, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)
	keyLeases, err := keyleasecontroller.NewKeyLeaseController(ds, ds.Logger)
	common.Must(err)
	janitorDatastore, err := janitordatastore.NewJanitorDatastore(ds, ds.Logger)
	common.Must(err)
	wpInstance, err := client.Stream[Rec](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)
	g, err := cd.RegisterGroup(ctx, tp.Id, group, consume.Beginning())
	common.Must(err)
	groupId = g.Id

	step("seed: two versions of user:1 -- the newer is the compaction head")
	publish(ctx, wpInstance, "user:1", 1)
	publish(ctx, wpInstance, "user:1", 2)
	staleID := scalarInt64(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.%s WHERE message_key = 'user:1'`, ds.Schema, stream.MessageLogTable(streamId)))
	headID := scalarInt64(ctx, fmt.Sprintf(`SELECT message_id FROM %s.%s WHERE compaction_key = 'user:1'`, ds.Schema, stream.CompactionHeadTable(streamId)))
	if staleID == headID {
		common.Die("seed broken: stale and head ids match")
	}
	fmt.Printf("  stale=%d head=%d\n", staleID, headID)

	step("stale message resolves superseded and never touches the lease row")
	c := claim(ctx, keyLeases, "user:1", staleID, 30*time.Second)
	if c.Verdict != keyleasecontroller.KeyLeaseSuperseded {
		common.Die(fmt.Sprintf("want superseded, got %s", c.Verdict))
	}
	if n := leaseCount(ctx); n != 0 {
		common.Die(fmt.Sprintf("superseded verdict created a lease row (count=%d) -- the head gate must run before the lease attempt", n))
	}
	fmt.Println("  ✓ superseded, zero lease rows")

	step("head message acquires when free; second attempt is busy")
	held := claim(ctx, keyLeases, "user:1", headID, 30*time.Second)
	if held.Verdict != keyleasecontroller.KeyLeaseAcquired || held.Token == uuid.Nil() {
		common.Die(fmt.Sprintf("want acquired with a token, got %s valid=%v", held.Verdict, held.Token != uuid.Nil()))
	}
	if c := claim(ctx, keyLeases, "user:1", headID, 30*time.Second); c.Verdict != keyleasecontroller.KeyLeaseBusy {
		common.Die(fmt.Sprintf("want busy while held, got %s", c.Verdict))
	}
	if n := leaseCount(ctx); n != 1 {
		common.Die(fmt.Sprintf("want exactly 1 lease row, got %d", n))
	}
	fmt.Println("  ✓ acquired, then busy")

	step("stale message still resolves superseded while the key is held (gate beats busy)")
	if c := claim(ctx, keyLeases, "user:1", staleID, 30*time.Second); c.Verdict != keyleasecontroller.KeyLeaseSuperseded {
		common.Die(fmt.Sprintf("want superseded (not busy) for a stale message on a held key, got %s", c.Verdict))
	}
	fmt.Println("  ✓ superseded takes precedence over busy")

	step("a release inside a rolled-back txn leaves the lease held")
	tx, err := ds.Pool.Begin(ctx)
	common.Must(err)
	// mirrors consumebase.release's SQL (pkg/consume/base/controller/datastore/keylease.go) -- keep in sync
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.%s
		WHERE consumer_group_id = $1
			AND message_key = $2
			AND token = $3;
	`, ds.Schema, stream.MessageKeyLeaseTable(streamId)), groupId, "user:1", held.Token)
	common.Must(err)
	if tag.RowsAffected() != 1 {
		common.Die("the in-txn release should have matched the held row")
	}
	common.Must(tx.Rollback(ctx))
	if c := claim(ctx, keyLeases, "user:1", headID, 30*time.Second); c.Verdict != keyleasecontroller.KeyLeaseBusy {
		common.Die(fmt.Sprintf("want busy after rolled-back release, got %s", c.Verdict))
	}
	fmt.Println("  ✓ rollback kept the lease")

	step("release frees the key for immediate reacquire")
	released, err := keyLeases.Release(ctx, held)
	common.Must(err)
	if !released {
		common.Die("release of the live holder should match its row")
	}
	short := claim(ctx, keyLeases, "user:1", headID, 300*time.Millisecond)
	if short.Verdict != keyleasecontroller.KeyLeaseAcquired {
		common.Die(fmt.Sprintf("want reacquire after release, got %s", short.Verdict))
	}
	fmt.Println("  ✓ released and reacquired")

	step("takeover only after expiry; the expired holder's release matches 0 rows")
	if c := claim(ctx, keyLeases, "user:1", headID, 30*time.Second); c.Verdict != keyleasecontroller.KeyLeaseBusy {
		common.Die(fmt.Sprintf("want busy before expiry, got %s", c.Verdict))
	}
	time.Sleep(400 * time.Millisecond)
	taker := claim(ctx, keyLeases, "user:1", headID, 30*time.Second)
	if taker.Verdict != keyleasecontroller.KeyLeaseAcquired {
		common.Die(fmt.Sprintf("want takeover after expiry, got %s", taker.Verdict))
	}
	if taker.Token == short.Token {
		common.Die("takeover must mint a fresh token")
	}
	staleReleased, err := keyLeases.Release(ctx, short)
	common.Must(err)
	if staleReleased {
		common.Die("the expired holder's release matched a row -- it must not delete the new holder's row")
	}
	if n := leaseCount(ctx); n != 1 {
		common.Die(fmt.Sprintf("the expired holder's release must not remove the new holder's row, count=%d", n))
	}
	takerReleased, err := keyLeases.Release(ctx, taker)
	common.Must(err)
	if !takerReleased {
		common.Die("the new holder's release should match its row")
	}
	fmt.Println("  ✓ busy before expiry, taken over after, expired token matched nothing")

	step("N concurrent acquires on one free key admit exactly one")
	const workers = 10
	var wg sync.WaitGroup
	results := make([]*keyleasecontroller.KeyLeaseClaim, workers)
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = claim(ctx, keyLeases, "user:1", headID, 30*time.Second)
		}()
	}
	wg.Wait()
	var winner *keyleasecontroller.KeyLeaseClaim
	acquired, busy := 0, 0
	for _, r := range results {
		switch r.Verdict {
		case keyleasecontroller.KeyLeaseAcquired:
			acquired++
			winner = r
		case keyleasecontroller.KeyLeaseBusy:
			busy++
		default:
			common.Die(fmt.Sprintf("unexpected verdict %s in the race", r.Verdict))
		}
	}
	if acquired != 1 || busy != workers-1 {
		common.Die(fmt.Sprintf("want exactly 1 winner, got acquired=%d busy=%d", acquired, busy))
	}
	released, err = keyLeases.Release(ctx, winner)
	common.Must(err)
	if !released {
		common.Die("race winner's release should match its row")
	}
	fmt.Printf("  ✓ %d racers, 1 winner\n", workers)

	step("old-then-new order: a newer head produced mid-hold waits for the release")
	publish(ctx, wpInstance, "user:3", 1)
	old3 := scalarInt64(ctx, fmt.Sprintf(`SELECT message_id FROM %s.%s WHERE compaction_key = 'user:3'`, ds.Schema, stream.CompactionHeadTable(streamId)))
	holding := claim(ctx, keyLeases, "user:3", old3, 30*time.Second)
	if holding.Verdict != keyleasecontroller.KeyLeaseAcquired {
		common.Die(fmt.Sprintf("want acquired on user:3, got %s", holding.Verdict))
	}
	publish(ctx, wpInstance, "user:3", 2)
	new3 := scalarInt64(ctx, fmt.Sprintf(`SELECT message_id FROM %s.%s WHERE compaction_key = 'user:3'`, ds.Schema, stream.CompactionHeadTable(streamId)))
	if c := claim(ctx, keyLeases, "user:3", new3, 30*time.Second); c.Verdict != keyleasecontroller.KeyLeaseBusy {
		common.Die(fmt.Sprintf("want busy for the new head while the old holds the key, got %s", c.Verdict))
	}
	if c := claim(ctx, keyLeases, "user:3", old3, 30*time.Second); c.Verdict != keyleasecontroller.KeyLeaseSuperseded {
		common.Die(fmt.Sprintf("want superseded for the held message now that the head moved, got %s", c.Verdict))
	}
	released, err = keyLeases.Release(ctx, holding)
	common.Must(err)
	if !released {
		common.Die("the old holder's release should match its row")
	}
	after := claim(ctx, keyLeases, "user:3", new3, 30*time.Second)
	if after.Verdict != keyleasecontroller.KeyLeaseAcquired {
		common.Die(fmt.Sprintf("want the new head to acquire after the release, got %s", after.Verdict))
	}
	released, err = keyLeases.Release(ctx, after)
	common.Must(err)
	if !released {
		common.Die("the new head's release should match its row")
	}
	fmt.Println("  ✓ new head waited out the old holder, then acquired")

	step("janitor sweep removes expired rows, leaves live ones")
	publish(ctx, wpInstance, "user:2", 1)
	head2 := scalarInt64(ctx, fmt.Sprintf(`SELECT message_id FROM %s.%s WHERE compaction_key = 'user:2'`, ds.Schema, stream.CompactionHeadTable(streamId)))
	expired := claim(ctx, keyLeases, "user:1", headID, 50*time.Millisecond)
	if expired.Verdict != keyleasecontroller.KeyLeaseAcquired {
		common.Die(fmt.Sprintf("sweep setup: want acquired, got %s", expired.Verdict))
	}
	live := claim(ctx, keyLeases, "user:2", head2, 30*time.Second)
	if live.Verdict != keyleasecontroller.KeyLeaseAcquired {
		common.Die(fmt.Sprintf("sweep setup: want acquired, got %s", live.Verdict))
	}
	time.Sleep(100 * time.Millisecond)
	common.Must(janitorDatastore.SweepExpiredKeyLeases(ctx, streamId, 1)) // batchSize 1 forces the batch loop
	if n := leaseCount(ctx); n != 1 {
		common.Die(fmt.Sprintf("want only the live row to survive the sweep, count=%d", n))
	}
	survivor := scalarString(ctx, fmt.Sprintf(`SELECT message_key FROM %s.%s WHERE consumer_group_id = $1`, ds.Schema, stream.MessageKeyLeaseTable(streamId)), groupId)
	if survivor != "user:2" {
		common.Die(fmt.Sprintf("sweep removed the wrong row, survivor=%s", survivor))
	}
	fmt.Println("  ✓ expired swept, live kept")

	step("destroying the stream drops its message_key_lease table")
	common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	var keyLeaseTable *string
	common.Must(ds.Pool.QueryRow(ctx, `SELECT to_regclass($1)::text;`, fmt.Sprintf("%s.%s", ds.Schema, stream.MessageKeyLeaseTable(streamId))).Scan(&keyLeaseTable))
	if keyLeaseTable != nil {
		common.Die("destroy left the message_key_lease table behind")
	}
	fmt.Println("  ✓ destroy dropped the table")

	fmt.Println("\n✅ KEY LEASE E2E TEST PASSED")
	return nil
}

func claim(ctx context.Context, cd *keyleasecontroller.KeyLeaseController, key string, msgID int64, d time.Duration) *keyleasecontroller.KeyLeaseClaim {
	c, err := cd.Claim(ctx, streamId, groupId, key, msgID, true, iCommon.ConcurrencyExclusive, keyleasecontroller.RangeBounds{}, d)
	common.Must(err)
	return c
}

func publish(ctx context.Context, wpInstance *sqlstreams.ProducerInstance[Rec], key string, version int) {
	_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*Rec, error) {
		return &Rec{Key: key, Version: version}, nil
	}, &sqlstreams.ProduceOptions{MessageKey: key, Compaction: &sqlstreams.CompactionOptions{Enable: true}})
	common.Must(err)
}

func leaseCount(ctx context.Context) int {
	var n int
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = $1`, ds.Schema, stream.MessageKeyLeaseTable(streamId)), groupId).Scan(&n))
	return n
}

func scalarInt64(ctx context.Context, q string, args ...any) int64 {
	var v int64
	common.Must(ds.Pool.QueryRow(ctx, q, args...).Scan(&v))
	return v
}

func scalarString(ctx context.Context, q string, args ...any) string {
	var v string
	common.Must(ds.Pool.QueryRow(ctx, q, args...).Scan(&v))
	return v
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
