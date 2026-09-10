package consume

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// invariant (monotonic cursor): committed advances to the lowest of the
// lowest live lease's low, the lowest unresolved exception's id minus one,
// and claimed; done, dead, and superseded rows do not hold it, and it never
// moves backward.
func TestAdvanceCommittedStopsAtTheLowestUnresolvedRow(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 6)
	claimed := claimRange(t, groups, consumer)
	cursors := newCursorAdvancerDatastore(t, groups)
	exceptions := newExceptionConsumerDatastore(t, groups)
	ctx := t.Context()

	// test
	committed, err := cursors.AdvanceCommitted(ctx, consumer.StreamId, consumer.Id)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if committed != 0 {
		t.Fatalf("AdvanceCommitted with lease (0, 6] live = %d, want 0", committed)
	}

	// test: the lease commits with an exception at 3 and a dead letter at 5,
	// beside a done row at 2 and a superseded row at 4
	outcomes := []datastore.Outcome{
		{MessageId: 3, Kind: datastore.OutcomeException, Err: "handler returned an error"},
		{MessageId: 5, Kind: datastore.OutcomeTerminal, Err: "handler returned a terminal error"},
	}
	if err := groups.Commit(ctx, consumer.StreamId, consumer.Id, claimed.Lease.Token, outcomes, 0, stream.DeliveryLogModeOff); err != nil {
		t.Fatal(err)
	}
	insertException(t, groups, consumer, 2, "done", -time.Hour, 0)
	insertException(t, groups, consumer, 4, "superseded", -time.Hour, 0)
	committed, err = cursors.AdvanceCommitted(ctx, consumer.StreamId, consumer.Id)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if committed != 2 {
		t.Fatalf("AdvanceCommitted with the exception at 3 unresolved = %d, want 2", committed)
	}

	// test: the exception at 3 resolves
	due, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)
	if err != nil || len(due) != 1 || due[0].MessageId != 3 {
		t.Fatalf("exception Claim = %+v, %v; want message 3", due, err)
	}
	if err := exceptions.RecordSuccess(ctx, &due[0], stream.DeliveryLogModeOff, nil); err != nil {
		t.Fatal(err)
	}
	committed, err = cursors.AdvanceCommitted(ctx, consumer.StreamId, consumer.Id)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if committed != 6 {
		t.Fatalf("AdvanceCommitted with every row resolved = %d, want claimed, 6", committed)
	}

	// test: an unresolved row appears below committed
	insertException(t, groups, consumer, 1, "ready", -time.Hour, 0)
	committed, err = cursors.AdvanceCommitted(ctx, consumer.StreamId, consumer.Id)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if committed != 6 {
		t.Fatalf("AdvanceCommitted with a ready row at 1 below committed = %d, want 6, never backward", committed)
	}
}
