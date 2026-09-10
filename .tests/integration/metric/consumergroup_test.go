package metric

import (
	"testing"
	"time"
)

// behavior: a consumer group snapshot counts the group's exception rows by
// status, dates the oldest row still to be resolved and ignores done and
// dead rows for it, counts the group's own claim leases, reads the log's
// head, and is (nil, nil) for a group with no cursor row.
func TestConsumerGroupSnapshotCountsTheGroupsOwnRowsByStatus(t *testing.T) {
	// setup
	metrics, orders := newMetricDatastore(t)
	ctx := t.Context()
	processor := registerConsumerGroup(t, metrics, orders, "processor")
	auditor := registerConsumerGroup(t, metrics, orders, "auditor")
	insertMessages(t, metrics, orders, 1, 5)
	setCursor(t, metrics, orders, processor.Id, 5, 0)
	insertException(t, metrics, orders, processor.Id, 1, "ready", 3*time.Hour)
	insertException(t, metrics, orders, processor.Id, 2, "inflight", 2*time.Hour)
	insertException(t, metrics, orders, processor.Id, 3, "deferred", time.Hour)
	insertException(t, metrics, orders, processor.Id, 4, "done", 5*time.Hour)
	insertException(t, metrics, orders, processor.Id, 5, "dead", 4*time.Hour)
	insertException(t, metrics, orders, auditor.Id, 1, "ready", 6*time.Hour)
	insertClaimLease(t, metrics, orders, processor.Id)
	insertClaimLease(t, metrics, orders, processor.Id)
	insertClaimLease(t, metrics, orders, auditor.Id)

	// test
	snapshot, err := metrics.ConsumerGroupSnapshot(ctx, orders.Id, processor.Id)
	missing, missingErr := metrics.ConsumerGroupSnapshot(ctx, orders.Id, 404)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Claimed != 5 || snapshot.Committed != 0 || snapshot.Head != 5 {
		t.Errorf("ConsumerGroupSnapshot(processor) cursor = (claimed %d, committed %d, head %d), want (5, 0, 5)", snapshot.Claimed, snapshot.Committed, snapshot.Head)
	}
	if snapshot.ReadyExceptions != 1 || snapshot.InflightExceptions != 1 || snapshot.DeferredExceptions != 1 || snapshot.DeadExceptions != 1 {
		t.Errorf("ConsumerGroupSnapshot(processor) exceptions = (ready %d, inflight %d, deferred %d, dead %d), want (1, 1, 1, 1)", snapshot.ReadyExceptions, snapshot.InflightExceptions, snapshot.DeferredExceptions, snapshot.DeadExceptions)
	}
	if snapshot.OldestUnresolvedAt == nil || time.Since(*snapshot.OldestUnresolvedAt) < 3*time.Hour-time.Minute || time.Since(*snapshot.OldestUnresolvedAt) > 3*time.Hour+time.Minute {
		t.Errorf("ConsumerGroupSnapshot(processor).OldestUnresolvedAt = %v, want the ready row from three hours ago, not the older done or dead rows", snapshot.OldestUnresolvedAt)
	}
	if snapshot.OpenLeases != 2 {
		t.Errorf("ConsumerGroupSnapshot(processor).OpenLeases = %d, want the group's own two leases", snapshot.OpenLeases)
	}
	if missing != nil || missingErr != nil {
		t.Errorf("ConsumerGroupSnapshot(404, a group with no cursor row) = %+v, %v; want nil, nil", missing, missingErr)
	}
}
