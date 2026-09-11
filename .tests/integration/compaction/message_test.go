package compaction

import (
	"testing"
	"time"
)

// Behavior: a key's history reads newest first within the limit, with rank 0
// and an empty routing key where the row has none; the storage-time window
// is inclusive at both ends and orders by created_at then id descending.
func TestKeyMessagesReadNewestFirstAndByInclusiveStorageTime(t *testing.T) {
	// setup
	heads, orders := newCompactionDatastore(t)
	ctx := t.Context()
	first := produceCompacted(t, heads, orders, "order-1", 3)
	second := produceUncompacted(t, heads, orders, "order-1")
	third := produceCompacted(t, heads, orders, "order-1", 1)
	produceCompacted(t, heads, orders, "order-2", 0)
	setCreatedAt(t, heads, orders, first, time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC))
	setCreatedAt(t, heads, orders, second, time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC))
	setCreatedAt(t, heads, orders, third, time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC))

	// test
	newest, err := heads.ListKeyMessages(ctx, orders.Id, "order-1", 2)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(newest) != 2 || newest[0].Id != third || newest[1].Id != second {
		t.Fatalf("ListKeyMessages(order-1, 2) = %+v, want ids [%d %d]", newest, third, second)
	}
	if newest[0].CompactionRank != 1 || newest[0].RoutingKey != "orders.updated" {
		t.Errorf("ListKeyMessages(order-1, 2)[0] = %+v, want rank 1 and routing key orders.updated", newest[0])
	}
	if newest[1].CompactionRank != 0 || newest[1].RoutingKey != "" {
		t.Errorf("ListKeyMessages(order-1, 2)[1] = %+v, want rank 0 and no routing key for the uncompacted row", newest[1])
	}

	// test
	atTwo, err := heads.ListKeyMessagesByCreatedAt(ctx, orders.Id, "order-1", time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC), time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	beforeTwo, err := heads.ListKeyMessagesByCreatedAt(ctx, orders.Id, "order-1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 1, 1, 30, 0, 0, time.UTC))

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(atTwo) != 2 || atTwo[0].Id != third || atTwo[1].Id != second {
		t.Errorf("ListKeyMessagesByCreatedAt(order-1, 02:00, 02:00) = %+v, want ids [%d %d]", atTwo, third, second)
	}
	if len(beforeTwo) != 1 || beforeTwo[0].Id != first {
		t.Errorf("ListKeyMessagesByCreatedAt(order-1, 00:00, 01:30) = %+v, want ids [%d]", beforeTwo, first)
	}
}
