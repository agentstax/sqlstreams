package stream

import (
	"slices"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
)

// behavior: the empty head sweep deletes a head that points at no message
// and has been idle past ttl, keeps a head pointing at a message however
// old, keeps a young empty head, and skips a row another transaction holds
// locked until that transaction ends.
func TestSweepExpiredEmptyCompactionHeadsDeletesOnlyIdleEmptyRows(t *testing.T) {
	// setup
	janitor, orders := newPartitionedJanitor(t)
	ctx := t.Context()
	heads := janitor.Datastore.Schema + "." + stream.CompactionHeadTable(orders.Id)
	insertEmptyCompactionHead(t, janitor, orders, "idle", 2*time.Hour)
	insertCompactionHead(t, janitor, orders, "filled", 1, 2*time.Hour)
	insertEmptyCompactionHead(t, janitor, orders, "young", 0)
	insertEmptyCompactionHead(t, janitor, orders, "locked", 2*time.Hour)
	holder := holdTransaction(t, janitor.Datastore.Pool)
	if _, err := holder.Exec(ctx, "SELECT compaction_key FROM "+heads+" WHERE compaction_key = 'locked' FOR UPDATE"); err != nil {
		t.Fatal(err)
	}

	// test
	err := janitor.SweepExpiredEmptyCompactionHeads(ctx, orders.Id, time.Hour, 10)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if keys := listKeys(t, janitor, stream.CompactionHeadTable(orders.Id), "compaction_key"); !slices.Equal(keys, []string{"filled", "locked", "young"}) {
		t.Errorf("compaction_head keys after SweepExpiredEmptyCompactionHeads(ttl 1h) = %v, want the idle empty head alone deleted [filled locked young]", keys)
	}

	// test: the lock holder ends
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	err = janitor.SweepExpiredEmptyCompactionHeads(ctx, orders.Id, time.Hour, 10)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if keys := listKeys(t, janitor, stream.CompactionHeadTable(orders.Id), "compaction_key"); !slices.Equal(keys, []string{"filled", "young"}) {
		t.Errorf("compaction_head keys after the lock holder ended = %v, want the locked head deleted too [filled young]", keys)
	}
}
