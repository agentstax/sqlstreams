package consume

import (
	"testing"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// invariant (declaration history): the sweep deletes a declarer's waiting
// rows older than ttl except its newest, never an installed row, and at most
// batchSize rows per sweep.
func TestWaitingDeclarationSweepKeepsTheNewestAndTheInstalled(t *testing.T) {
	// setup: one installed row, then three waiting rows from one declarer
	groups, consumer := newMessageConsumerDatastore(t)
	consumers := newConsumeDatastore(t, groups)
	janitor := newJanitorDatastore(t, groups)
	log := groups.Datastore.Schema + "." + stream.BindingConfigLogTable(consumer.StreamId)
	ctx := t.Context()
	if _, err := consumers.DeclareBindings(ctx, consumer.StreamId, consumer.Id, []string{"orders.*"}, "host-a", time.Now()); err != nil {
		t.Fatal(err)
	}
	declareLiveInstance(t, groups, consumer)
	for range 3 {
		outcome, err := consumers.DeclareBindings(ctx, consumer.StreamId, consumer.Id, []string{"shipments.*"}, "host-b", time.Now())
		if err != nil || outcome != consume.BindingWaiting {
			t.Fatalf("DeclareBindings during setup = %s, %v; want waiting", outcome, err)
		}
	}

	// test
	fresh, err := janitor.SweepExpiredWaitingDeclarations(ctx, time.Hour, 10)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if fresh != 0 {
		t.Fatalf("SweepExpiredWaitingDeclarations(1h) over rows attempted now = %d, want 0", fresh)
	}

	// test: every row is older than ttl, swept one at a time
	if _, err := groups.Datastore.Pool.Exec(ctx, "UPDATE "+log+" SET attempted_at = now() - interval '2 hours'"); err != nil {
		t.Fatal(err)
	}
	first, firstErr := janitor.SweepExpiredWaitingDeclarations(ctx, time.Hour, 1)
	second, secondErr := janitor.SweepExpiredWaitingDeclarations(ctx, time.Hour, 10)

	// verify
	if firstErr != nil || secondErr != nil {
		t.Fatalf("SweepExpiredWaitingDeclarations twice = %v, %v; want nil, nil", firstErr, secondErr)
	}
	if first != 1 || second != 1 {
		t.Fatalf("sweeps with batch 1 then batch 10 = %d, %d; want 1, 1 -- the newest waiting row stays", first, second)
	}
	var installed, waiting int
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE status = 'installed'), count(*) FILTER (WHERE status = 'waiting') FROM "+log+" WHERE consumer_group_id = $1", consumer.Id).Scan(&installed, &waiting); err != nil {
		t.Fatal(err)
	}
	if installed != 1 || waiting != 1 {
		t.Fatalf("binding log rows after the sweeps = (installed %d, waiting %d), want (1, 1)", installed, waiting)
	}
}
