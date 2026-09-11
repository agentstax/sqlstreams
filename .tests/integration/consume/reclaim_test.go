package consume

import (
	"errors"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// invariant (one live lease): a claim after a lease expired takes the same
// range under a new token, so the worker that let it expire cannot commit
// it -- its commit is ErrLeaseLost -- and the new holder's commit advances
// the cursor over the range.
func TestReclaimOfAnExpiredLeaseRotatesTheToken(t *testing.T) {
	// setup: the first worker claims (0, 4] and never commits
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 4)
	expired := claimRange(t, groups, consumer)
	expireClaimLease(t, groups, consumer)
	cursors := newCursorAdvancerDatastore(t, groups)
	ctx := t.Context()

	// test
	reclaimed, err := groups.ClaimMessagesWithCursor(ctx, consumer.StreamId, consumer.Id, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed == nil || reclaimed.Lease.Low != 0 || reclaimed.Lease.High != 4 || len(reclaimed.Messages) != 4 {
		t.Fatalf("ClaimMessagesWithCursor after the lease expired = %+v, want the same range (0, 4] with its four messages", reclaimed)
	}
	if reclaimed.Lease.Token == expired.Lease.Token {
		t.Errorf("reclaimed lease token = %v, want a token other than the expired lease's", reclaimed.Lease.Token)
	}
	if reclaimed.Lease.Reclaims != 1 {
		t.Errorf("reclaimed lease reclaims = %d, want 1", reclaimed.Lease.Reclaims)
	}

	// test: the first worker resurrects and commits, then the new holder commits
	staleErr := groups.Commit(ctx, consumer.StreamId, consumer.Id, expired.Lease.Token, nil, time.Minute, stream.DeliveryLogModeFailures)
	commitErr := groups.Commit(ctx, consumer.StreamId, consumer.Id, reclaimed.Lease.Token, nil, time.Minute, stream.DeliveryLogModeFailures)

	// verify
	if !errors.Is(staleErr, common.ErrLeaseLost) {
		t.Errorf("Commit with the expired lease's token = %v, want ErrLeaseLost", staleErr)
	}
	if commitErr != nil {
		t.Fatalf("Commit with the reclaimed lease's token = %v, want nil", commitErr)
	}
	committed, err := cursors.AdvanceCommitted(ctx, consumer.StreamId, consumer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if committed != 4 {
		t.Errorf("AdvanceCommitted after the reclaimed range committed = %d, want 4", committed)
	}
}
