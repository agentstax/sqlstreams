package consume

import (
	"testing"
	"time"
	"uuid"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	keyleasedatastore "github.com/allegedlyreliable/sqlstreams/pkg/consume/base/controller/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5/pgtype"
)

// invariant (one delivery per key): a group holds at most one live lease
// per message key -- a second claimant is busy, the holder's own token
// re-takes the lease, an expired lease is taken over, and a release with a
// token the lease does not hold frees nothing.
func TestOneLiveKeyLeasePerGroupAndKey(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	keys := newKeyLeaseDatastore(t, groups)
	leases := groups.Datastore.Schema + "." + stream.MessageKeyLeaseTable(consumer.StreamId)
	holder := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	other := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	ctx := t.Context()

	// test
	first, firstErr := keys.Claim(ctx, consumer.StreamId, consumer.Id, "order-1", 1, false, common.ConcurrencyExclusive, 0, 0, time.Minute, holder)
	second, secondErr := keys.Claim(ctx, consumer.StreamId, consumer.Id, "order-1", 2, false, common.ConcurrencyExclusive, 0, 0, time.Minute, other)
	retaken, retakenErr := keys.Claim(ctx, consumer.StreamId, consumer.Id, "order-1", 1, false, common.ConcurrencyExclusive, 0, 0, time.Minute, holder)

	// verify
	if firstErr != nil || secondErr != nil || retakenErr != nil {
		t.Fatalf("key lease Claim three times = %v, %v, %v; want nil, nil, nil", firstErr, secondErr, retakenErr)
	}
	if first.Verdict != keyleasedatastore.KeyLeaseAcquired {
		t.Errorf("first Claim of the key = %s, want acquired", first.Verdict)
	}
	if second.Verdict != keyleasedatastore.KeyLeaseBusy {
		t.Errorf("Claim of a held key with another token = %s, want busy", second.Verdict)
	}
	if retaken.Verdict != keyleasedatastore.KeyLeaseAcquired {
		t.Errorf("Claim of a held key with the holder's token = %s, want acquired", retaken.Verdict)
	}

	// test: a release with the wrong token, then the lease expires
	released, err := keys.Release(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := groups.Datastore.Pool.Exec(ctx, "UPDATE "+leases+" SET expires_at = now() - interval '1 second' WHERE consumer_group_id = $1 AND message_key = 'order-1'", consumer.Id); err != nil {
		t.Fatal(err)
	}
	takenOver, err := keys.Claim(ctx, consumer.StreamId, consumer.Id, "order-1", 2, false, common.ConcurrencyExclusive, 0, 0, time.Minute, other)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if released {
		t.Errorf("Release with a token the lease does not hold = true, want false")
	}
	if takenOver.Verdict != keyleasedatastore.KeyLeaseAcquired {
		t.Errorf("Claim of an expired key with another token = %s, want acquired", takenOver.Verdict)
	}
}

// invariant (compaction): a claim for a compacted message that is no longer
// its key's head is superseded and leaves no lease row for the key.
func TestCompactedKeyLeaseClaimBehindTheHeadIsSuperseded(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceKeyedMessages(t, groups, consumer, "order-1", 2)
	keys := newKeyLeaseDatastore(t, groups)
	heads := groups.Datastore.Schema + "." + stream.CompactionHeadTable(consumer.StreamId)
	leases := groups.Datastore.Schema + "." + stream.MessageKeyLeaseTable(consumer.StreamId)
	ctx := t.Context()
	if _, err := groups.Datastore.Pool.Exec(ctx, "INSERT INTO "+heads+" (compaction_key, message_id, schema_version, compaction_rank) VALUES ('order-1', 2, 1, 0)"); err != nil {
		t.Fatal(err)
	}

	// test
	claim, err := keys.Claim(ctx, consumer.StreamId, consumer.Id, "order-1", 1, true, common.ConcurrencyExclusive, 0, 0, time.Minute, pgtype.UUID{Bytes: uuid.New(), Valid: true})

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if claim.Verdict != keyleasedatastore.KeyLeaseSuperseded || claim.Token.Valid {
		t.Fatalf("compacted Claim of message 1 behind head 2 = %+v, want superseded with no token", claim)
	}
	var held int
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+leases+" WHERE consumer_group_id = $1 AND message_key = 'order-1'", consumer.Id).Scan(&held); err != nil {
		t.Fatal(err)
	}
	if held != 0 {
		t.Fatalf("key lease rows after a superseded claim = %d, want 0", held)
	}
}

// invariant (per-key order): an ordered claim is busy while an earlier
// same-key message sits above the group's committed cursor and outside the
// caller's own range; the same message inside the caller's range does not
// block it.
func TestOrderedKeyLeaseClaimWaitsForAnEarlierMessageOutsideItsRange(t *testing.T) {
	// setup: the group has committed nothing, so both messages are above its cursor
	groups, consumer := newMessageConsumerDatastore(t)
	produceKeyedMessages(t, groups, consumer, "order-1", 2)
	keys := newKeyLeaseDatastore(t, groups)
	token := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	ctx := t.Context()

	// test
	outside, outsideErr := keys.Claim(ctx, consumer.StreamId, consumer.Id, "order-1", 2, false, common.ConcurrencyOrdered, 1, 2, time.Minute, token)
	inside, insideErr := keys.Claim(ctx, consumer.StreamId, consumer.Id, "order-1", 2, false, common.ConcurrencyOrdered, 0, 2, time.Minute, token)

	// verify
	if outsideErr != nil || insideErr != nil {
		t.Fatalf("ordered Claim twice = %v, %v; want nil, nil", outsideErr, insideErr)
	}
	if outside.Verdict != keyleasedatastore.KeyLeaseBusy {
		t.Errorf("ordered Claim of message 2 with message 1 outside range (1, 2] = %s, want busy", outside.Verdict)
	}
	if inside.Verdict != keyleasedatastore.KeyLeaseAcquired {
		t.Errorf("ordered Claim of message 2 with message 1 inside range (0, 2] = %s, want acquired", inside.Verdict)
	}
}
