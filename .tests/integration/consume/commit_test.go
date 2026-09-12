package consume

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume/messageconsumer/controller/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5/pgtype"
)

// invariant (one live lease): a commit frees its lease before it writes any
// outcome, so a commit with a token the lease no longer holds returns
// ErrLeaseLost and writes no exception rows.
func TestCommitWithStaleTokenWritesNoOutcomes(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 2)
	claimed := claimRange(t, groups, consumer)
	pool := groups.Datastore.Pool
	queue := groups.Datastore.Schema + "." + stream.ExceptionQueueTable(consumer.StreamId)
	leases := groups.Datastore.Schema + "." + stream.ClaimLeaseTable(consumer.StreamId)
	stale := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	outcomes := []datastore.Outcome{{MessageId: 1, Kind: datastore.OutcomeException, Err: "handler returned an error"}}
	ctx := t.Context()

	// test
	err := groups.Commit(ctx, consumer.StreamId, consumer.Id, stale, outcomes, time.Second, stream.DeliveryLogModeFailures)

	// verify
	if !errors.Is(err, common.ErrLeaseLost) {
		t.Fatalf("Commit with a stale token = %v, want ErrLeaseLost", err)
	}
	var exceptions int
	var live int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+queue+" WHERE consumer_group_id = $1", consumer.Id).Scan(&exceptions); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+leases+" WHERE token = $1", claimed.Lease.Token).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if exceptions != 0 || live != 1 {
		t.Fatalf("exception rows, live leases after a stale commit = %d, %d; want 0, 1", exceptions, live)
	}
}

// behavior: one commit resolves every outcome kind -- exception and delayed
// wait out their backoff as 'ready', terminal is 'dead', deferred is
// claimable at once, superseded and success leave no queue row -- and every
// kind writes one log row at attempt 0.
func TestCommitWritesOneQueueRowPerOutcomeKind(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 6)
	claimed := claimRange(t, groups, consumer)
	exceptions := newExceptionConsumerDatastore(t, groups)
	metrics := newMetricDatastore(t, groups)
	queue := groups.Datastore.Schema + "." + stream.ExceptionQueueTable(consumer.StreamId)
	logs := groups.Datastore.Schema + "." + stream.DeliveryLogTable(consumer.StreamId)
	outcomes := []datastore.Outcome{
		{MessageId: 1, Kind: datastore.OutcomeException, Err: "handler returned an error"},
		{MessageId: 2, Kind: datastore.OutcomeTerminal, Err: "handler returned a terminal error"},
		{MessageId: 3, Kind: datastore.OutcomeDeferred},
		{MessageId: 4, Kind: datastore.OutcomeDelayed, Delay: time.Hour},
		{MessageId: 5, Kind: datastore.OutcomeSuperseded},
		{MessageId: 6, Kind: datastore.OutcomeSuccess},
	}
	ctx := t.Context()

	// test
	err := groups.Commit(ctx, consumer.StreamId, consumer.Id, claimed.Lease.Token, outcomes, time.Minute, stream.DeliveryLogModeAll)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := metrics.ConsumerGroupSnapshot(ctx, consumer.StreamId, consumer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ReadyExceptions != 2 || snapshot.DeadExceptions != 1 || snapshot.DeferredExceptions != 1 || snapshot.InflightExceptions != 0 {
		t.Fatalf("exceptions after Commit = (ready %d, dead %d, deferred %d, inflight %d), want (2, 1, 1, 0)", snapshot.ReadyExceptions, snapshot.DeadExceptions, snapshot.DeferredExceptions, snapshot.InflightExceptions)
	}
	dueNow, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)
	if err != nil {
		t.Fatal(err)
	}
	if len(dueNow) != 1 || dueNow[0].MessageId != 3 {
		t.Fatalf("exception Claim right after Commit = %+v, want only the deferred message 3", dueNow)
	}
	var logged int
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE consumer_group_id = $1 AND attempt = 0", consumer.Id).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 6 {
		t.Fatalf("delivery log rows at attempt 0 after Commit = %d, want one per outcome, 6", logged)
	}

	// test: the backoffs pass
	if _, err := groups.Datastore.Pool.Exec(ctx, "UPDATE "+queue+" SET can_run_after = now() WHERE consumer_group_id = $1", consumer.Id); err != nil {
		t.Fatal(err)
	}
	dueLater, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(dueLater) != 2 || dueLater[0].MessageId != 1 || dueLater[1].MessageId != 4 {
		t.Fatalf("exception Claim after the backoffs = %+v, want messages 1 and 4", dueLater)
	}
	if dueLater[0].Delays != 0 || dueLater[1].Delays != 1 {
		t.Fatalf("delays of messages 1 and 4 = %d, %d; want 0, 1", dueLater[0].Delays, dueLater[1].Delays)
	}
}

// invariant (one live lease): a partial commit narrows the lease to the
// unprocessed suffix under the same token -- the committed cursor can advance
// to the last processed id, and the same token still commits the rest.
func TestPartialCommitNarrowsTheLeaseUnderTheSameToken(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 2)
	claimed := claimRange(t, groups, consumer)
	cursors := newCursorAdvancerDatastore(t, groups)
	ctx := t.Context()

	// test
	err := groups.PartialCommit(ctx, consumer.StreamId, consumer.Id, claimed.Lease.Token, 1, nil, time.Minute, stream.DeliveryLogModeFailures)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	committed, err := cursors.AdvanceCommitted(ctx, consumer.StreamId, consumer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if committed != 1 {
		t.Fatalf("AdvanceCommitted after PartialCommit(1) = %d, want 1", committed)
	}

	// test: the same token commits the suffix
	err = groups.Commit(ctx, consumer.StreamId, consumer.Id, claimed.Lease.Token, nil, time.Minute, stream.DeliveryLogModeFailures)

	// verify
	if err != nil {
		t.Fatalf("Commit with the partial commit's token = %v, want nil", err)
	}
	committed, err = cursors.AdvanceCommitted(ctx, consumer.StreamId, consumer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if committed != 2 {
		t.Fatalf("AdvanceCommitted after the suffix committed = %d, want 2", committed)
	}
}
