package stream

import (
	"slices"
	"testing"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// Invariant: expiry follows creation timestamps rather than caller UUID order,
// drains multiple batches in one sweep, and preserves young keys.
func TestSweepIdempotencyKeysUsesCreationTime(t *testing.T) {
	// setup
	janitor, orders := newRetentionJanitor(t)
	ctx := t.Context()
	keys := janitor.Datastore.Schema + "." + stream.IdempotencyKeyTable(orders.Id)
	seedRetentionKeys(t, janitor, keys)

	// test
	err := janitor.SweepExpiredIdempotencyKeys(ctx, orders.Id, time.Hour, 2)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	var remaining []string
	if err := janitor.Datastore.Pool.QueryRow(ctx, "SELECT array_agg(idempotency_key::text ORDER BY idempotency_key) FROM "+keys).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(remaining, []string{"00000000-0000-0000-0000-000000000001"}) {
		t.Fatalf("SweepExpiredIdempotencyKeys(1h, batch=2) retained %v, want [00000000-0000-0000-0000-000000000001]", remaining)
	}

	// test
	if _, err := janitor.Datastore.Pool.Exec(ctx, "UPDATE "+keys+" SET created_at=now()-interval '2 hours'"); err != nil {
		t.Fatal(err)
	}
	err = janitor.SweepExpiredIdempotencyKeys(ctx, orders.Id, time.Hour, 2)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := janitor.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+keys).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("SweepExpiredIdempotencyKeys(all expired) retained %d, want 0", count)
	}
}
