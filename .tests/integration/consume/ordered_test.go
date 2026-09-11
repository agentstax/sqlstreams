package consume

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/pkg/common"
	keyleasedatastore "github.com/agentstax/sqlstreams/pkg/consume/base/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5/pgtype"
)

// invariant (per-key order): a failed ordered message holds its key's lane
// -- the deferred successor is neither claimed nor leased -- and a terminal
// failure releases it: the cursor passes the dead row and the successor is
// claimable and leasable.
func TestDeadOrderedPredecessorReleasesItsKeysLane(t *testing.T) {
	// setup: message 1 failed and message 2 was deferred behind it
	groups, consumer := newMessageConsumerDatastore(t)
	produceKeyedMessages(t, groups, consumer, "acct-1", 2)
	claimed := claimRange(t, groups, consumer)
	exceptions := newExceptionConsumerDatastore(t, groups)
	keys := newKeyLeaseDatastore(t, groups)
	cursors := newCursorAdvancerDatastore(t, groups)
	queue := groups.Datastore.Schema + "." + stream.ExceptionQueueTable(consumer.StreamId)
	token := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	outcomes := []datastore.Outcome{
		{MessageId: 1, MessageKey: "acct-1", Concurrency: common.ConcurrencyOrdered, Kind: datastore.OutcomeException, Err: "ledger unavailable"},
		{MessageId: 2, MessageKey: "acct-1", Concurrency: common.ConcurrencyOrdered, Kind: datastore.OutcomeDeferred},
	}
	ctx := t.Context()
	if err := groups.Commit(ctx, consumer.StreamId, consumer.Id, claimed.Lease.Token, outcomes, time.Hour, stream.DeliveryLogModeFailures); err != nil {
		t.Fatal(err)
	}

	// test
	due, dueErr := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeFailures)
	held, heldErr := keys.Claim(ctx, consumer.StreamId, consumer.Id, "acct-1", 2, false, common.ConcurrencyOrdered, 0, 0, time.Minute, token)

	// verify
	if dueErr != nil || heldErr != nil {
		t.Fatalf("exception Claim, key lease Claim behind the failed row = %v, %v; want nil, nil", dueErr, heldErr)
	}
	if len(due) != 0 {
		t.Errorf("exception Claim behind the failed row = %+v, want nothing, the successor is deferred", due)
	}
	if held.Verdict != keyleasedatastore.KeyLeaseBusy {
		t.Errorf("ordered Claim of message 2 behind the failed message 1 = %s, want busy", held.Verdict)
	}

	// test: message 1 comes due and fails terminally
	if _, err := groups.Datastore.Pool.Exec(ctx, "UPDATE "+queue+" SET can_run_after = now() WHERE consumer_group_id = $1 AND message_id = 1", consumer.Id); err != nil {
		t.Fatal(err)
	}
	failed, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeFailures)
	if err != nil || len(failed) != 1 || failed[0].MessageId != 1 {
		t.Fatalf("exception Claim once message 1 is due = %+v, %v; want message 1 alone", failed, err)
	}
	if err := exceptions.RecordTerminal(ctx, &failed[0], errors.New("account closed"), stream.DeliveryLogModeFailures, nil); err != nil {
		t.Fatal(err)
	}
	committed, committedErr := cursors.AdvanceCommitted(ctx, consumer.StreamId, consumer.Id)
	successor, successorErr := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeFailures)
	released, releasedErr := keys.Claim(ctx, consumer.StreamId, consumer.Id, "acct-1", 2, false, common.ConcurrencyOrdered, 0, 0, time.Minute, token)

	// verify
	if committedErr != nil || successorErr != nil || releasedErr != nil {
		t.Fatalf("AdvanceCommitted, exception Claim, key lease Claim after the dead letter = %v, %v, %v; want nil, nil, nil", committedErr, successorErr, releasedErr)
	}
	if committed != 1 {
		t.Errorf("AdvanceCommitted past the dead row = %d, want 1", committed)
	}
	if len(successor) != 1 || successor[0].MessageId != 2 {
		t.Errorf("exception Claim behind the dead row = %+v, want the deferred message 2", successor)
	}
	if released.Verdict != keyleasedatastore.KeyLeaseAcquired {
		t.Errorf("ordered Claim of message 2 behind the dead message 1 = %s, want acquired", released.Verdict)
	}
}
