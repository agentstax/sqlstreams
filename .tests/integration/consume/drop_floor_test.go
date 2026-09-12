package consume

import (
	"testing"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// invariant (no stall): a claim whose range lies in a dropped partition
// returns no messages but still moves the claimed cursor past the hole, so
// the next claim reads the messages that survive.
func TestClaimOverADroppedPartitionAdvancesPastTheHole(t *testing.T) {
	// setup: ids 1-3 fill partitions 0 and 1, dropped before the group reads
	groups, consumer, registered := newPartitionedMessageConsumerDatastore(t)
	seedPartitionedMessages(t, groups, registered, 6)
	dropPartition(t, groups, registered, 1, 3)
	ctx := t.Context()

	// test
	hole, err := groups.ClaimMessagesWithCursor(ctx, consumer.StreamId, consumer.Id, 1, 3, 3, time.Minute, stream.DeliveryLogModeFailures)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if hole == nil || hole.Lease.Low != 0 || hole.Lease.High != 3 || len(hole.Messages) != 0 {
		t.Fatalf("ClaimMessagesWithCursor over the dropped partitions = %+v, want lease (0, 3] with no messages", hole)
	}

	// test: the hole commits and the group reads on
	if err := groups.Commit(ctx, consumer.StreamId, consumer.Id, hole.Lease.Token, nil, time.Minute, stream.DeliveryLogModeFailures); err != nil {
		t.Fatal(err)
	}
	next, err := groups.ClaimMessagesWithCursor(ctx, consumer.StreamId, consumer.Id, 1, 3, 3, time.Minute, stream.DeliveryLogModeFailures)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.Lease.Low != 3 || next.Lease.High != 6 {
		t.Fatalf("ClaimMessagesWithCursor after the hole = %+v, want lease (3, 6]", next)
	}
	if ids := messageIds(next.Messages); len(ids) != 3 || ids[0] != 4 || ids[1] != 5 || ids[2] != 6 {
		t.Errorf("messages after the hole = %v, want the surviving [4 5 6]", ids)
	}
}
