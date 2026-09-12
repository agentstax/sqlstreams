package compaction

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/compaction/controller/datastore"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/jackc/pgx/v5/pgconn"
)

// Invariant: one read-modify-produce per key at a time -- LockHead creates
// the key's lockable row, returns nil while the key has no head, and holds
// the row lock until its transaction ends.
func TestLockHeadHoldsTheKeyUntilItsTransactionEnds(t *testing.T) {
	// setup
	heads, orders := newCompactionDatastore(t)
	ctx := t.Context()
	first := holdTransaction(t, heads.Datastore.Pool)
	second := holdTransaction(t, heads.Datastore.Pool)
	setLockTimeout(t, second, 100*time.Millisecond)

	// test
	head, err := heads.LockHead(ctx, first, orders.Id, "order-1")

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if head != nil {
		t.Errorf("LockHead(order-1) on a new key = %+v, want nil", head)
	}

	// test
	_, err = heads.LockHead(ctx, second, orders.Id, "order-1")

	// verify
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("LockHead(order-1) while another transaction holds it error = %v, want lock_not_available (55P03)", err)
	}

	// test
	if err := first.Raw().Commit(ctx); err != nil {
		t.Fatal(err)
	}
	third := holdTransaction(t, heads.Datastore.Pool)
	head, err = heads.LockHead(ctx, third, orders.Id, "order-1")

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if head != nil {
		t.Errorf("LockHead(order-1) after the holder committed = %+v, want nil", head)
	}
}

// Behavior: a compacted produce inside the locking transaction fills the
// row LockHead created, so the key's head is the produced message.
func TestCompactedProduceFillsTheRowLockHeadCreated(t *testing.T) {
	// setup
	heads, orders := newCompactionDatastore(t)
	ctx := t.Context()
	produces := newProduceDatastore(t, heads)
	var locked *datastore.MessageLogRow
	var producedId int64

	// test
	err := iDatastore.InTransaction(ctx, heads.Datastore, func(ctx context.Context, tx iDatastore.Tx) error {
		var err error
		locked, err = heads.LockHead(ctx, tx, orders.Id, "order-1")
		if err != nil {
			return err
		}
		produced, err := produces.AppendMessageInTx(ctx, tx, orders.Id, orders.PartitionSize, produceCompactionTestMessage, compactedAppend("order-1", 0))
		if err != nil {
			return err
		}
		producedId = produced.Id
		return nil
	})

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if locked != nil {
		t.Errorf("LockHead(order-1) before the produce = %+v, want nil", locked)
	}
	head, err := heads.GetHead(ctx, orders.Id, "order-1")
	if err != nil {
		t.Fatal(err)
	}
	if head == nil || head.Id != producedId {
		t.Errorf("GetHead(order-1) after the commit = %+v, want the produced message %d", head, producedId)
	}
}

// Behavior: GetHead and ListHeads read only keys with a head -- an empty
// lockable row and an unseen key are nil and absent.
func TestGetHeadAndListHeadsSkipEmptyRowsAndAbsentKeys(t *testing.T) {
	// setup
	heads, orders := newCompactionDatastore(t)
	ctx := t.Context()
	aId := produceCompacted(t, heads, orders, "order-a", 2)
	lockEmptyHead(t, heads, orders, "order-b")
	cId := produceCompacted(t, heads, orders, "order-c", 0)

	// test
	a, err := heads.GetHead(ctx, orders.Id, "order-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := heads.GetHead(ctx, orders.Id, "order-b")
	if err != nil {
		t.Fatal(err)
	}
	d, err := heads.GetHead(ctx, orders.Id, "order-d")
	if err != nil {
		t.Fatal(err)
	}
	listed, err := heads.ListHeads(ctx, orders.Id)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if b != nil {
		t.Errorf("GetHead(order-b) on an empty row = %+v, want nil", b)
	}
	if d != nil {
		t.Errorf("GetHead(order-d) on an unseen key = %+v, want nil", d)
	}
	if a == nil {
		t.Fatal("GetHead(order-a) = nil, want the head")
	}
	var payload compactionTestMessage
	if err := json.Unmarshal(a.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if a.Id != aId || a.MessageKey != "order-a" || a.RoutingKey != "orders.updated" || a.CompactionRank != 2 || payload.Kind != "created" {
		t.Errorf("GetHead(order-a) = %+v with payload %+v, want id %d, key order-a, routing key orders.updated, rank 2, kind created", a, payload, aId)
	}
	if len(listed) != 2 || listed[0].Id != aId || listed[1].Id != cId {
		t.Errorf("ListHeads() = %+v, want the heads of order-a (%d) then order-c (%d)", listed, aId, cId)
	}
}
