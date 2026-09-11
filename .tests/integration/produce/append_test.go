package produce

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"golang.org/x/sync/errgroup"
)

// Invariant: idempotent produce -- a second append under the same
// idempotency key stores nothing and reports the duplicate; a keyless
// message stores NULL keys, never empty strings.
func TestAppendMessageUnderARepeatedKeyIsADuplicate(t *testing.T) {
	// setup
	produces, orders := newProduceDatastore(t)
	ctx := t.Context()
	messages := produces.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	claims := produces.Datastore.Schema + "." + stream.IdempotencyKeyTable(orders.Id)
	key := uuid.New()

	// test
	first, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, plainAppend(key))
	if err != nil {
		t.Fatal(err)
	}
	second, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, plainAppend(key))

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if first.Duplicate || first.Id == 0 {
		t.Errorf("AppendMessage(first) = %+v, want a new id and Duplicate false", first)
	}
	if !second.Duplicate || second.Id != 0 || second.Message == nil {
		t.Errorf("AppendMessage(repeat) = %+v, want Duplicate true, Id 0, and the payload", second)
	}
	if count := countRows(t, produces, messages); count != 1 {
		t.Errorf("message_log rows after the repeat = %d, want 1", count)
	}
	if count := countRows(t, produces, claims); count != 1 {
		t.Errorf("idempotency_key rows after the repeat = %d, want 1", count)
	}
	var routingKey *string
	var messageKey *string
	if err := produces.Datastore.Pool.QueryRow(ctx, "SELECT routing_key, message_key FROM "+messages+" WHERE id = $1", first.Id).Scan(&routingKey, &messageKey); err != nil {
		t.Fatal(err)
	}
	if routingKey != nil || messageKey != nil {
		t.Errorf("stored keys of a keyless message = %v, %v, want NULL, NULL", routingKey, messageKey)
	}
}

// Invariant: a produce whose ProducerFunc fails leaves no claim row, so the
// caller's retry under the same key lands instead of reporting a duplicate.
func TestAppendMessageWhoseProducerFailsLeavesNoClaim(t *testing.T) {
	// setup
	produces, orders := newProduceDatastore(t)
	ctx := t.Context()
	claims := produces.Datastore.Schema + "." + stream.IdempotencyKeyTable(orders.Id)
	key := uuid.New()

	// test
	_, err := produces.AppendMessage(ctx, orders.Id, partitionSize, failingProducer, plainAppend(key))

	// verify
	if !errors.Is(err, errProducerFailed) {
		t.Fatalf("AppendMessage(failing producer) error = %v, want errProducerFailed", err)
	}
	if count := countRows(t, produces, claims); count != 0 {
		t.Errorf("idempotency_key rows after the failed produce = %d, want 0", count)
	}

	// test
	retried, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, plainAppend(key))

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if retried.Duplicate || retried.Id == 0 {
		t.Errorf("AppendMessage(retry under the same key) = %+v, want a new id and Duplicate false", retried)
	}
}

// Invariant: a compacted produce keeps compaction_head at the key's winner --
// within a rank the newest id, across ranks the highest rank.
func TestCompactedAppendKeepsTheHeadAtTheWinner(t *testing.T) {
	// setup
	produces, orders := newProduceDatastore(t)
	ctx := t.Context()

	// test
	_, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, compactedAppend("order-1", 0))
	if err != nil {
		t.Fatal(err)
	}
	second, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, compactedAppend("order-1", 0))

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if headId, rank := readHead(t, produces, orders, "order-1"); headId != second.Id || rank != 0 {
		t.Errorf("head(order-1) after two rank-0 produces = (%d, %d), want (%d, 0)", headId, rank, second.Id)
	}

	// test
	third, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, compactedAppend("order-1", 5))
	if err != nil {
		t.Fatal(err)
	}
	_, err = produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, compactedAppend("order-1", 3))

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if headId, rank := readHead(t, produces, orders, "order-1"); headId != third.Id || rank != 5 {
		t.Errorf("head(order-1) after a rank-5 then a rank-3 produce = (%d, %d), want (%d, 5)", headId, rank, third.Id)
	}

	// test
	fifth, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, compactedAppend("order-1", 5))

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if headId, rank := readHead(t, produces, orders, "order-1"); headId != fifth.Id || rank != 5 {
		t.Errorf("head(order-1) after a second rank-5 produce = (%d, %d), want (%d, 5)", headId, rank, fifth.Id)
	}
}

// Behavior: an append whose id has no partition creates the covering
// partition and lands in it; the skipped partition is never created.
func TestAppendMessageCreatesTheMissingPartitionItLandsIn(t *testing.T) {
	// setup
	produces, orders := newProduceDatastore(t)
	ctx := t.Context()
	advanceSequence(t, produces, orders, 2*partitionSize-1)

	// test
	appended, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, plainAppend(uuid.New()))

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if appended.Id/partitionSize != 2 {
		t.Errorf("AppendMessage() past two partitions landed at id %d, want an id in partition 2", appended.Id)
	}
	if !partitionExists(t, produces, orders, 2) {
		t.Error("partition 2 after the append = missing, want created")
	}
	if partitionExists(t, produces, orders, 1) {
		t.Error("partition 1 after the append = created, want missing")
	}
	if ids := listMessageIds(t, produces, orders); !slices.Equal(ids, []int64{appended.Id}) {
		t.Errorf("message ids after the append = %v, want [%d]", ids, appended.Id)
	}
}

// Invariant: concurrent appends racing to create the same missing partition
// all land -- no producer sees the loser's duplicate-table error.
func TestConcurrentAppendsIntoAMissingPartitionAllLand(t *testing.T) {
	// setup
	produces, orders := newProduceDatastore(t)
	ctx := t.Context()
	advanceSequence(t, produces, orders, 2*partitionSize-1)
	const producers = 8
	ids := make(chan int64, producers)

	// test
	var group errgroup.Group
	for range producers {
		group.Go(func() error {
			appended, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, plainAppend(uuid.New()))
			if err != nil {
				return err
			}
			ids <- appended.Id
			return nil
		})
	}
	err := group.Wait()

	// verify
	if err != nil {
		t.Fatal(err)
	}
	close(ids)
	var landed []int64
	for id := range ids {
		if id/partitionSize != 2 {
			t.Errorf("AppendMessage() landed at id %d, want an id in partition 2", id)
		}
		landed = append(landed, id)
	}
	slices.Sort(landed)
	if stored := listMessageIds(t, produces, orders); !slices.Equal(stored, landed) {
		t.Errorf("message ids after the concurrent appends = %v, want the returned ids %v", stored, landed)
	}
	if !partitionExists(t, produces, orders, 2) {
		t.Error("partition 2 after the concurrent appends = missing, want created")
	}
}

// Invariant: an append inside the caller's transaction heals a missing
// partition within its own savepoint, so the caller's earlier write and the
// message commit together.
func TestAppendMessageInTxHealsInsideASavepointAndCommitsWithTheCaller(t *testing.T) {
	// setup
	produces, orders := newProduceDatastore(t)
	ctx := t.Context()
	heads := produces.Datastore.Schema + "." + stream.CompactionHeadTable(orders.Id)
	claims := produces.Datastore.Schema + "." + stream.IdempotencyKeyTable(orders.Id)
	advanceSequence(t, produces, orders, 2*partitionSize-1)
	var appended int64

	// test
	err := iDatastore.InTransaction(ctx, produces.Datastore, func(ctx context.Context, tx iDatastore.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO "+heads+" (compaction_key) VALUES ('marker')"); err != nil {
			return err
		}
		result, err := produces.AppendMessageInTx(ctx, tx, orders.Id, partitionSize, produceTestMessageFunc, plainAppend(uuid.New()))
		if err != nil {
			return err
		}
		appended = result.Id
		return nil
	})

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if appended/partitionSize != 2 {
		t.Errorf("AppendMessageInTx() past two partitions landed at id %d, want an id in partition 2", appended)
	}
	if !partitionExists(t, produces, orders, 2) {
		t.Error("partition 2 after the commit = missing, want created")
	}
	if ids := listMessageIds(t, produces, orders); !slices.Equal(ids, []int64{appended}) {
		t.Errorf("message ids after the commit = %v, want [%d]", ids, appended)
	}
	if count := countRows(t, produces, claims); count != 1 {
		t.Errorf("idempotency_key rows after the commit = %d, want 1", count)
	}
	if count := countRows(t, produces, heads); count != 1 {
		t.Errorf("compaction_head rows after the commit = %d, want the caller's marker row", count)
	}
}

// Invariant: a compacted produce of a newer message schema takes the head
// from an older schema's message whatever the ranks, and an older schema's
// message never takes it back.
func TestCompactedAppendKeepsTheHeadAtTheNewestSchemaVersion(t *testing.T) {
	// setup: the older schema holds the head at a high rank
	produces, orders := newProduceDatastore(t)
	ctx := t.Context()
	if _, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, compactedAppend("order-1", 5)); err != nil {
		t.Fatal(err)
	}

	// test
	newer, err := produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageV2Func, compactedAppendV2("order-1", 0))

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if headId, rank := readHead(t, produces, orders, "order-1"); headId != newer.Id || rank != 0 {
		t.Errorf("head(order-1) after a schema 2 rank-0 produce over a schema 1 rank-5 head = (%d, %d), want (%d, 0)", headId, rank, newer.Id)
	}

	// test: the older schema produces again at a higher rank
	_, err = produces.AppendMessage(ctx, orders.Id, partitionSize, produceTestMessageFunc, compactedAppend("order-1", 9))

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if headId, rank := readHead(t, produces, orders, "order-1"); headId != newer.Id || rank != 0 {
		t.Errorf("head(order-1) after a schema 1 rank-9 produce behind a schema 2 head = (%d, %d), want the schema 2 head kept (%d, 0)", headId, rank, newer.Id)
	}
}
