package stream

import (
	"slices"
	"testing"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// behavior: the key lease sweep deletes only expired rows, drains more than
// one batch in a pass, and keeps a live lease -- the row a crashed consumer
// leaves on a key that never gets another message has no other way out.
func TestSweepExpiredKeyLeasesDeletesOnlyExpiredRows(t *testing.T) {
	// setup
	janitor, orders := newPartitionedJanitor(t)
	ctx := t.Context()
	claimKeyLease(t, janitor, orders, "order-1")
	claimKeyLease(t, janitor, orders, "order-2")
	claimKeyLease(t, janitor, orders, "order-3")
	expireKeyLease(t, janitor, orders, "order-1")
	expireKeyLease(t, janitor, orders, "order-2")

	// test
	err := janitor.SweepExpiredKeyLeases(ctx, orders.Id, 1)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if keys := listKeys(t, janitor, stream.MessageKeyLeaseTable(orders.Id), "message_key"); !slices.Equal(keys, []string{"order-3"}) {
		t.Errorf("message_key_lease keys after SweepExpiredKeyLeases(batch 1) = %v, want the live lease alone [order-3]", keys)
	}
}
