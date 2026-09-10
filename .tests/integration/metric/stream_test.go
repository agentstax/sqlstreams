package metric

import (
	"reflect"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/metric/controller/datastore"
)

// behavior: a group's lag at a payload version counts the rows at that
// version above its own committed cursor, and its ready, inflight, and
// deferred exception rows at that version -- never done or dead rows, never
// rows at another version.
func TestConsumerGroupSchemaVersionLagCountsOnlyTheVersionAboveEachCursor(t *testing.T) {
	// setup
	metrics, orders := newMetricDatastore(t)
	ctx := t.Context()
	processor := registerConsumerGroup(t, metrics, orders, "processor")
	auditor := registerConsumerGroup(t, metrics, orders, "auditor")
	insertMessages(t, metrics, orders, 1, 3)
	insertMessages(t, metrics, orders, 2, 2)
	setCursor(t, metrics, orders, processor.Id, 2, 2)
	insertException(t, metrics, orders, processor.Id, 1, "ready", 0)
	insertException(t, metrics, orders, processor.Id, 2, "done", 0)
	insertException(t, metrics, orders, processor.Id, 3, "dead", 0)
	insertException(t, metrics, orders, processor.Id, 4, "inflight", 0)
	insertException(t, metrics, orders, auditor.Id, 5, "deferred", 0)

	// test
	atOne, oneErr := metrics.ConsumerGroupSchemaVersionLag(ctx, orders.Id, 1)
	atTwo, twoErr := metrics.ConsumerGroupSchemaVersionLag(ctx, orders.Id, 2)

	// verify
	if oneErr != nil {
		t.Fatal(oneErr)
	}
	wantOne := []datastore.ConsumerGroupSchemaVersionLagRow{
		{ConsumerGroup: "auditor", Unconsumed: 3, UnresolvedExceptions: 0},
		{ConsumerGroup: "processor", Unconsumed: 1, UnresolvedExceptions: 1},
	}
	if !reflect.DeepEqual(atOne, wantOne) {
		t.Errorf("ConsumerGroupSchemaVersionLag(version 1) = %+v, want %+v", atOne, wantOne)
	}
	if twoErr != nil {
		t.Fatal(twoErr)
	}
	wantTwo := []datastore.ConsumerGroupSchemaVersionLagRow{
		{ConsumerGroup: "auditor", Unconsumed: 2, UnresolvedExceptions: 1},
		{ConsumerGroup: "processor", Unconsumed: 2, UnresolvedExceptions: 1},
	}
	if !reflect.DeepEqual(atTwo, wantTwo) {
		t.Errorf("ConsumerGroupSchemaVersionLag(version 2) = %+v, want %+v", atTwo, wantTwo)
	}
}

// behavior: a stream snapshot counts the log's partitions, reads compacted
// as false with no head row pointing at a message, and once heads exist
// counts the rows without a head with the oldest one's age.
func TestStreamSnapshotReadsPartitionsAndHeadlessCompactionRows(t *testing.T) {
	// setup
	metrics, orders := newMetricDatastore(t)
	ctx := t.Context()
	insertMessages(t, metrics, orders, 1, 2)

	// test
	before, beforeErr := metrics.StreamSnapshot(ctx, orders.Id)

	// verify
	if beforeErr != nil {
		t.Fatal(beforeErr)
	}
	if before.Partitions != 1 || before.Compacted || before.CompactionRowsWithoutHead != 0 || before.OldestCompactionRowWithoutHeadSecs != 0 {
		t.Errorf("StreamSnapshot(orders) with no head rows = %+v, want one partition, not compacted, no headless rows", before)
	}

	// test: one head points at a message, two point at none
	insertCompactionHead(t, metrics, orders, "order-1", 1)
	insertEmptyCompactionHead(t, metrics, orders, "order-2", 2*time.Hour)
	insertEmptyCompactionHead(t, metrics, orders, "order-3", time.Hour)
	after, afterErr := metrics.StreamSnapshot(ctx, orders.Id)

	// verify
	if afterErr != nil {
		t.Fatal(afterErr)
	}
	if after.Partitions != 1 || !after.Compacted || after.CompactionRowsWithoutHead != 2 {
		t.Errorf("StreamSnapshot(orders) with head rows = %+v, want one partition, compacted, two headless rows", after)
	}
	if after.OldestCompactionRowWithoutHeadSecs < 7200-60 || after.OldestCompactionRowWithoutHeadSecs > 7200+60 {
		t.Errorf("StreamSnapshot(orders).OldestCompactionRowWithoutHeadSecs = %v, want about two hours, 7200", after.OldestCompactionRowWithoutHeadSecs)
	}
}
