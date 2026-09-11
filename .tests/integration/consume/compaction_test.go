package consume

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
)

// invariant (compaction): a claim over several versions of a key returns
// only the key's head, on the cursor path and the lifecycle path alike; an
// uncompacted message always comes back; the older versions stay in the log
// and the cursor path's lease still covers them.
func TestClaimsReturnOnlyTheCompactionHeadOfEachKey(t *testing.T) {
	// setup: user-1 at ids 1-3, an unkeyed message at 4, user-2 at 5-6
	groups, consumer := newMessageConsumerDatastore(t)
	produceCompactedMessages(t, groups, consumer, "user-1", 3)
	produceMessages(t, groups, consumer, 1)
	produceCompactedMessages(t, groups, consumer, "user-2", 2)
	lifecycle := registerConsumer(t, groups, consumer, "auditors")
	deliveries := newDeliveryConsumerDatastore(t, groups)
	messages := groups.Datastore.Schema + "." + stream.MessageLogTable(consumer.StreamId)
	ctx := t.Context()

	// test
	claimed, err := groups.ClaimMessagesWithCursor(ctx, consumer.StreamId, consumer.Id, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil || claimed.Lease.Low != 0 || claimed.Lease.High != 6 {
		t.Fatalf("ClaimMessagesWithCursor over three keys = %+v, want lease (0, 6]", claimed)
	}
	if ids := messageIds(claimed.Messages); len(ids) != 3 || ids[0] != 3 || ids[1] != 4 || ids[2] != 6 {
		t.Errorf("ClaimMessagesWithCursor messages = %v, want the heads and the unkeyed message [3 4 6]", ids)
	}
	var stored int
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+messages).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 6 {
		t.Errorf("message_log rows after the claim = %d, want every version kept, 6", stored)
	}

	// test: the lifecycle path over the same log
	if err := deliveries.FanOut(ctx, lifecycle.StreamId, lifecycle.Id, 1, 100); err != nil {
		t.Fatal(err)
	}
	delivered, err := deliveries.ClaimMessagesWithLifecycle(ctx, lifecycle.StreamId, lifecycle.Id, 100)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(delivered) != 3 || delivered[0].MessageId != 3 || delivered[1].MessageId != 4 || delivered[2].MessageId != 6 {
		t.Errorf("ClaimMessagesWithLifecycle after FanOut = %+v, want deliveries for [3 4 6] only", delivered)
	}
}
