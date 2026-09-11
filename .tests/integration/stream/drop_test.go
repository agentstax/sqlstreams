package stream

import (
	"slices"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
)

// invariant (retention): a drop pass never touches the active partition,
// drops an older partition once its newest row is past ttl, keeps one whose
// newest row is younger, and drops nothing at ttl 0 -- retention disabled.
func TestDropExpiredPartitionsJudgesEachOlderPartitionByItsNewestRow(t *testing.T) {
	// setup
	janitor, orders := newPartitionedJanitor(t)
	ctx := t.Context()
	seedMessages(t, janitor, orders, 8)
	ageMessages(t, janitor, orders, 1, 3)

	// test
	err := janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, 0, true, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if partitions := listPartitions(t, janitor, orders, 4); !slices.Equal(partitions, []int64{0, 1, 2, 3, 4}) {
		t.Errorf("partitions after DropExpiredPartitions(ttl 0) = %v, want every one kept [0 1 2 3 4]", partitions)
	}

	// test: partitions 0 and 1 are past ttl, 2 and 3 are not
	err = janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, true, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if partitions := listPartitions(t, janitor, orders, 4); !slices.Equal(partitions, []int64{2, 3, 4}) {
		t.Errorf("partitions after DropExpiredPartitions(ttl 1h) = %v, want the expired 0 and 1 dropped [2 3 4]", partitions)
	}

	// test: every row is past ttl, the active partition included
	ageMessages(t, janitor, orders, 4, 8)
	err = janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, true, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if partitions := listPartitions(t, janitor, orders, 4); !slices.Equal(partitions, []int64{4}) {
		t.Errorf("partitions after DropExpiredPartitions(ttl 1h, all expired) = %v, want only the active partition [4]", partitions)
	}
}

// invariant (no loss): a drop pass keeps every expired partition a lagging
// group has not committed past, drops those the cursor has passed, and
// ignores the cursor under allowDropPastCommitted.
func TestDropExpiredPartitionsStopsAtTheLaggingCursor(t *testing.T) {
	// setup
	janitor, orders := newPartitionedJanitor(t)
	ctx := t.Context()
	seedMessages(t, janitor, orders, 8)
	ageMessages(t, janitor, orders, 1, 8)

	// test: the group has committed nothing
	err := janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, false, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if partitions := listPartitions(t, janitor, orders, 4); !slices.Equal(partitions, []int64{0, 1, 2, 3, 4}) {
		t.Errorf("partitions after DropExpiredPartitions(committed 0) = %v, want every one kept [0 1 2 3 4]", partitions)
	}

	// test: the group has committed through id 3, the last id of partition 1
	commitCursor(t, janitor, orders, 3)
	err = janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, false, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if partitions := listPartitions(t, janitor, orders, 4); !slices.Equal(partitions, []int64{2, 3, 4}) {
		t.Errorf("partitions after DropExpiredPartitions(committed 3) = %v, want 0 and 1 dropped [2 3 4]", partitions)
	}

	// test: the cursor is ignored
	err = janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, true, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if partitions := listPartitions(t, janitor, orders, 4); !slices.Equal(partitions, []int64{4}) {
		t.Errorf("partitions after DropExpiredPartitions(allowDropPastCommitted) = %v, want only the active partition [4]", partitions)
	}
}

// invariant (no orphans): a partition drop deletes the exception_queue,
// delivery_log, and compaction_head rows in the partition's id range and
// leaves rows outside it; DeliveryLogModeOff leaves delivery_log alone.
func TestDropPartitionDeletesTheRowsInItsIdRange(t *testing.T) {
	// setup
	janitor, orders := newPartitionedJanitor(t)
	ctx := t.Context()
	seedMessages(t, janitor, orders, 8)
	insertDeliveryRows(t, janitor, orders, 1)
	insertDeliveryRows(t, janitor, orders, 2)
	insertDeliveryRows(t, janitor, orders, 5)
	ageMessages(t, janitor, orders, 1, 3)

	// test: partitions 0 and 1 drop with the delivery log off
	err := janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, true, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if partitions := listPartitions(t, janitor, orders, 4); !slices.Equal(partitions, []int64{2, 3, 4}) {
		t.Fatalf("partitions after the first drop = %v, want [2 3 4]", partitions)
	}
	if ids := listMessageIds(t, janitor, stream.ExceptionQueueTable(orders.Id)); !slices.Equal(ids, []int64{5}) {
		t.Errorf("exception_queue message ids after the first drop = %v, want [5]", ids)
	}
	if ids := listMessageIds(t, janitor, stream.DeliveryLogTable(orders.Id)); !slices.Equal(ids, []int64{1, 2, 5}) {
		t.Errorf("delivery_log message ids after the first drop with the log off = %v, want every row kept [1 2 5]", ids)
	}
	if ids := listMessageIds(t, janitor, stream.CompactionHeadTable(orders.Id)); !slices.Equal(ids, []int64{5}) {
		t.Errorf("compaction_head message ids after the first drop = %v, want [5]", ids)
	}

	// test: partition 2 drops with the delivery log on
	ageMessages(t, janitor, orders, 4, 5)
	err = janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, true, stream.DeliveryLogModeFailures)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if partitions := listPartitions(t, janitor, orders, 4); !slices.Equal(partitions, []int64{3, 4}) {
		t.Fatalf("partitions after the second drop = %v, want [3 4]", partitions)
	}
	if ids := listMessageIds(t, janitor, stream.ExceptionQueueTable(orders.Id)); len(ids) != 0 {
		t.Errorf("exception_queue message ids after the second drop = %v, want none", ids)
	}
	if ids := listMessageIds(t, janitor, stream.DeliveryLogTable(orders.Id)); !slices.Equal(ids, []int64{1, 2}) {
		t.Errorf("delivery_log message ids after the second drop = %v, want only the rows the earlier drop left [1 2]", ids)
	}
	if ids := listMessageIds(t, janitor, stream.CompactionHeadTable(orders.Id)); len(ids) != 0 {
		t.Errorf("compaction_head message ids after the second drop = %v, want none", ids)
	}
}

// invariant (idempotent retry): a drop pass rerun with the same arguments
// after one already dropped its partitions -- a retry after an ambiguous
// commit -- succeeds and drops nothing more.
func TestDropExpiredPartitionsRunTwiceDropsNothingMore(t *testing.T) {
	// setup
	janitor, orders := newPartitionedJanitor(t)
	ctx := t.Context()
	seedMessages(t, janitor, orders, 8)
	ageMessages(t, janitor, orders, 1, 3)
	if err := janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, true, stream.DeliveryLogModeOff); err != nil {
		t.Fatal(err)
	}

	// test
	err := janitor.DropExpiredPartitions(ctx, orders.Id, orders.PartitionSize, time.Hour, true, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatalf("DropExpiredPartitions(ttl 1h) rerun error = %v, want nil", err)
	}
	if partitions := listPartitions(t, janitor, orders, 4); !slices.Equal(partitions, []int64{2, 3, 4}) {
		t.Errorf("partitions after the rerun = %v, want the first pass's result kept [2 3 4]", partitions)
	}
}
