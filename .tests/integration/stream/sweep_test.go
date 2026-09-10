package stream

import (
	"github.com/agentstax/sqlstreams/pkg/stream"
	"slices"
	"testing"
	"time"
)

// Invariant: row cleanup preserves every row within TTL plus grace, including
// rows beside an overdue row, and still respects consumer progress.
func TestPartialSweepPreservesRowsWithinGrace(t *testing.T) {
	// setup
	janitor, orders := newRetentionJanitor(t)
	ctx := t.Context()
	messages := janitor.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	cursor := janitor.Datastore.Schema + "." + stream.ConsumerGroupCursorTable(orders.Id)
	if _, err := janitor.Datastore.Pool.Exec(ctx, "INSERT INTO "+messages+" (id,schema_version,payload,created_at) SELECT id,1,'{}',CASE WHEN id<=5 THEN now()-interval '75 minutes' ELSE now() END FROM generate_series(1,6) id"); err != nil {
		t.Fatal(err)
	}
	if _, err := janitor.Datastore.Pool.Exec(ctx, "UPDATE "+cursor+" SET committed=4"); err != nil {
		t.Fatal(err)
	}

	// test
	err := janitor.SweepExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, 30*time.Minute, false, 2, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	if err := janitor.Datastore.Pool.QueryRow(ctx, "SELECT array_agg(id ORDER BY id) FROM "+messages).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []int64{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("SweepExpiredPartitions(within grace) retained %v, want [1 2 3 4 5 6]", ids)
	}

	// test
	if _, err := janitor.Datastore.Pool.Exec(ctx, "UPDATE "+messages+" SET created_at=now()-interval '2 hours' WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	err = janitor.SweepExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, 30*time.Minute, false, 2, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if err := janitor.Datastore.Pool.QueryRow(ctx, "SELECT array_agg(id ORDER BY id) FROM "+messages).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []int64{2, 3, 4, 5, 6}) {
		t.Fatalf("SweepExpiredPartitions(rows within grace) retained %v, want [2 3 4 5 6]", ids)
	}

	// test
	if _, err := janitor.Datastore.Pool.Exec(ctx, "UPDATE "+messages+" SET created_at=now()-interval '2 hours' WHERE id IN (2,3,4,5)"); err != nil {
		t.Fatal(err)
	}
	err = janitor.SweepExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, 30*time.Minute, false, 2, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if err := janitor.Datastore.Pool.QueryRow(ctx, "SELECT array_agg(id ORDER BY id) FROM "+messages).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []int64{5, 6}) {
		t.Fatalf("SweepExpiredPartitions(overdue rows, committed=4) retained %v, want [5 6]", ids)
	}

	// test
	if _, err := janitor.Datastore.Pool.Exec(ctx, "UPDATE "+messages+" SET created_at=now()-interval '2 hours' WHERE id=5"); err != nil {
		t.Fatal(err)
	}
	err = janitor.SweepExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, 30*time.Minute, true, 2, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if err := janitor.Datastore.Pool.QueryRow(ctx, "SELECT array_agg(id ORDER BY id) FROM "+messages).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []int64{6}) {
		t.Fatalf("SweepExpiredPartitions(overdue sparse partition, ignore cursor) retained %v, want [6]", ids)
	}
}

// Invariant: zero-grace cleanup drains multiple batches through the committed
// cursor, preserves young and uncommitted rows, and resumes as the cursor advances.
func TestZeroPartialSweepGracePreservesUnexpiredAndUncommittedMessages(t *testing.T) {
	// setup
	janitor, orders := newRetentionJanitor(t)
	ctx := t.Context()
	messages := janitor.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	cursor := janitor.Datastore.Schema + "." + stream.ConsumerGroupCursorTable(orders.Id)
	seedRetentionMessages(t, janitor, messages, cursor)

	// test
	err := janitor.SweepExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, 0, false, 2, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	if err := janitor.Datastore.Pool.QueryRow(ctx, "SELECT array_agg(id ORDER BY id) FROM "+messages).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []int64{4, 5, 6, 7, 8, 9, 10}) {
		t.Fatalf("SweepExpiredPartitions(zero grace, committed=3, batch=2) retained %v, want [4 5 6 7 8 9 10]", ids)
	}

	// test
	if _, err := janitor.Datastore.Pool.Exec(ctx, "UPDATE "+cursor+" SET committed=10"); err != nil {
		t.Fatal(err)
	}
	err = janitor.SweepExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, 0, false, 2, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if err := janitor.Datastore.Pool.QueryRow(ctx, "SELECT array_agg(id ORDER BY id) FROM "+messages).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []int64{5, 6, 7, 8, 9, 10}) {
		t.Fatalf("SweepExpiredPartitions(zero grace, committed=10) retained %v, want [5 6 7 8 9 10]", ids)
	}
}

// Invariant: grace affects only row sweeps; whole expired partitions still drop.
func TestPartialSweepGraceDoesNotDelayWholePartitionDrop(t *testing.T) {
	// setup
	janitor, orders := newRetentionJanitor(t)
	ctx := t.Context()
	messages := janitor.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	partition := janitor.Datastore.Schema + "." + stream.MessageLogPartitionTable(orders.Id, 0)
	fillRetentionPartitions(t, janitor)
	if _, err := janitor.Datastore.Pool.Exec(ctx, "DELETE FROM "+messages+" WHERE id NOT IN (1,1001)"); err != nil {
		t.Fatal(err)
	}
	if _, err := janitor.Datastore.Pool.Exec(ctx, "UPDATE "+messages+" SET created_at=now()-interval '75 minutes' WHERE id=1"); err != nil {
		t.Fatal(err)
	}

	// test
	err := janitor.SweepExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, 30*time.Minute, true, 2, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := janitor.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+messages).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("SweepExpiredPartitions(within grace) retained %d, want 2", count)
	}

	// test
	err = janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, true, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	var dropped bool
	if err := janitor.Datastore.Pool.QueryRow(ctx, "SELECT to_regclass($1) IS NULL", partition).Scan(&dropped); err != nil {
		t.Fatal(err)
	}
	if !dropped {
		t.Fatalf("DropExpiredPartitions(%s within row grace) dropped %v, want true", partition, dropped)
	}
}
