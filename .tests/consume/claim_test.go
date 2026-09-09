package consume

import (
	"context"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// invariant (no loss): a claim that finds nothing safe to deliver still
// records the head and transaction fence it observed, so the next claim can
// deliver those messages once the older transactions finish, even while a
// newer transaction is open.
func TestEmptyClaimPersistsPendingObservation(t *testing.T) {
	// setup
	test := newClaimTest(t)
	ctx := t.Context()
	if _, err := test.pool.Exec(ctx, "INSERT INTO "+test.messages+" (schema_version, payload) VALUES (1, '{}'), (1, '{}')"); err != nil {
		t.Fatal(err)
	}
	older := holdTransaction(t, test.pool)
	if _, err := test.pool.Exec(ctx, "SELECT pg_current_xact_id()"); err != nil {
		t.Fatal(err)
	}

	// test
	claimed, err := test.claim(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// verify
	if claimed != nil {
		t.Fatalf("claim() with an older transaction open = %+v, want nil", claimed)
	}
	var head int64
	var recorded bool
	if err := test.pool.QueryRow(ctx, "SELECT pending_head, pending_xid IS NOT NULL FROM "+test.cursor+" WHERE consumer_group_id = $1", test.groupId).Scan(&head, &recorded); err != nil {
		t.Fatal(err)
	}
	if head != 2 || !recorded {
		t.Fatalf("cursor after empty claim = (pending_head %d, pending_xid recorded %t), want (2, true)", head, recorded)
	}

	// test: the older transaction finishes and a new one opens
	if err := older.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	holdTransaction(t, test.pool)
	if _, err := test.pool.Exec(ctx, "SELECT pg_current_xact_id()"); err != nil {
		t.Fatal(err)
	}
	claimed, err = test.claim(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// verify
	if claimed == nil || len(claimed.Messages) != 2 || claimed.Lease.Low != 0 || claimed.Lease.High != 2 {
		t.Fatalf("claim() after the older transaction committed = %+v, want both messages in lease (0, 2]", claimed)
	}
}

// invariant (no loss): a producer whose transaction id is beyond the claim's
// snapshot xmax, and whose message id is below a committed one, is never
// skipped -- the claim waits and then delivers both in id order.
func TestClaimWaitsForProducerBeyondSnapshotXmax(t *testing.T) {
	// setup: both producers own transaction ids before allocating message
	// ids, as the idempotency write requires; their allocation orders differ
	test := newClaimTest(t)
	ctx := t.Context()
	older := holdTransaction(t, test.pool)
	newer := holdTransaction(t, test.pool)
	if _, err := newer.Exec(ctx, "INSERT INTO "+test.messages+" (schema_version, payload) VALUES (1, '{}')"); err != nil {
		t.Fatal(err)
	}
	if _, err := older.Exec(ctx, "INSERT INTO "+test.messages+" (schema_version, payload) VALUES (1, '{}')"); err != nil {
		t.Fatal(err)
	}
	if err := older.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var outside bool
	if err := newer.QueryRow(ctx, "SELECT pg_current_xact_id() >= pg_snapshot_xmax(pg_current_snapshot())").Scan(&outside); err != nil {
		t.Fatal(err)
	}
	if !outside {
		t.Fatal("another completed transaction advanced snapshot xmax -- the test needs a server nothing else is writing to")
	}

	// test
	claimed, err := test.claim(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// verify
	if claimed != nil {
		t.Fatalf("claim() with message 1 uncommitted = lease (%d, %d] over %+v, want nil", claimed.Lease.Low, claimed.Lease.High, claimed.Messages)
	}

	// test: the newer producer commits
	if err := newer.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	claimed, err = test.claim(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// verify
	if claimed == nil || len(claimed.Messages) != 2 || claimed.Messages[0].Id != 1 || claimed.Messages[1].Id != 2 {
		t.Fatalf("claim() after both commits = %+v, want messages 1 and 2 in order", claimed)
	}
}

// invariant: a caught-up group's polls allocate no transaction ids, so an
// idle fleet does not advance the cluster toward xid wraparound.
func TestCaughtUpClaimsDoNotAllocateTransactionIds(t *testing.T) {
	// setup
	test := newClaimTest(t)
	ctx := t.Context()
	var before, after int64
	if err := test.pool.QueryRow(ctx, "SELECT pg_current_xact_id()::text::bigint").Scan(&before); err != nil {
		t.Fatal(err)
	}

	// test
	for range 3 {
		claimed, err := test.claim(ctx)
		if err != nil || claimed != nil {
			t.Fatalf("claim() on a caught-up group = %+v, %v; want nil, nil", claimed, err)
		}
	}

	// verify: the read of before allocated the one id between the two reads
	if err := test.pool.QueryRow(ctx, "SELECT pg_current_xact_id()::text::bigint").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before+1 {
		t.Fatalf("transaction ids allocated by three caught-up claims = %d, want 0", after-before-1)
	}
}

func (c *claimTest) claim(ctx context.Context) (*datastore.ClaimedRange, error) {
	return c.groups.ClaimMessagesWithCursor(ctx, c.streamId, c.groupId, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)
}
