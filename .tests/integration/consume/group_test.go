package consume

import (
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume/messageconsumer/controller/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5/pgtype"
)

// invariant (idempotent registration): registering a group again returns
// the existing row and never moves its cursor; a first registration at the
// head places claimed, committed, and settled head at the log's last id.
func TestRegisterGroupPlacesTheCursorOnceAtCreation(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 3)
	consumers := newConsumeDatastore(t, groups)
	metrics := newMetricDatastore(t, groups)
	cursor := groups.Datastore.Schema + "." + stream.ConsumerGroupCursorTable(consumer.StreamId)
	ctx := t.Context()

	// test
	created, createdErr := consumers.RegisterGroup(ctx, consumer.StreamId, "auditors", consume.CursorPosition{Kind: consume.CursorPositionHead})
	again, againErr := consumers.RegisterGroup(ctx, consumer.StreamId, "auditors", consume.CursorPosition{})

	// verify
	if createdErr != nil || againErr != nil {
		t.Fatalf("RegisterGroup twice = %v, %v; want nil, nil", createdErr, againErr)
	}
	if again.Id != created.Id {
		t.Fatalf("second RegisterGroup(auditors) = row %d, want the existing row %d", again.Id, created.Id)
	}
	snapshot, err := metrics.ConsumerGroupSnapshot(ctx, consumer.StreamId, created.Id)
	if err != nil {
		t.Fatal(err)
	}
	var settledHead int64
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT settled_head FROM "+cursor+" WHERE consumer_group_id = $1", created.Id).Scan(&settledHead); err != nil {
		t.Fatal(err)
	}
	if snapshot.Claimed != 3 || snapshot.Committed != 3 || settledHead != 3 {
		t.Fatalf("cursor of a group registered at the head, then again from the beginning = (claimed %d, committed %d, settled_head %d), want (3, 3, 3)", snapshot.Claimed, snapshot.Committed, settledHead)
	}
}

// behavior: a first binding declaration installs its patterns; the same set
// joins and writes nothing; a different set waits while an instance is live,
// appending a log row and keeping the installed patterns; once no instance
// is live it installs.
func TestDeclareBindingsInstallsJoinsOrWaits(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	consumers := newConsumeDatastore(t, groups)
	bindings := groups.Datastore.Schema + "." + stream.BindingConfigTable(consumer.StreamId)
	instances := groups.Datastore.Schema + ".worker_instance"
	ctx := t.Context()

	// test
	installed, installedErr := consumers.DeclareBindings(ctx, consumer.StreamId, consumer.Id, []string{"orders.*"}, "host-a", time.Now())
	joined, joinedErr := consumers.DeclareBindings(ctx, consumer.StreamId, consumer.Id, []string{"orders.*"}, "host-b", time.Now())

	// verify
	if installedErr != nil || joinedErr != nil {
		t.Fatalf("DeclareBindings twice = %v, %v; want nil, nil", installedErr, joinedErr)
	}
	if installed != consume.BindingInstalled || joined != consume.BindingJoined {
		t.Fatalf("first declaration, same set again = %s, %s; want installed, joined", installed, joined)
	}
	log, err := consumers.ListGroupBindingConfigLog(ctx, consumer.StreamId, consumer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 1 || log[0].Status != "installed" {
		t.Fatalf("binding log after install and join = %+v, want the one installed row", log)
	}

	// test: a different set while an instance is live
	declareLiveInstance(t, groups, consumer)
	waiting, err := consumers.DeclareBindings(ctx, consumer.StreamId, consumer.Id, []string{"shipments.*"}, "host-b", time.Now())

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if waiting != consume.BindingWaiting {
		t.Fatalf("different set with a live instance = %s, want waiting", waiting)
	}
	var patterns []string
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT array_agg(pattern ORDER BY pattern) FROM "+bindings+" WHERE consumer_group_id = $1", consumer.Id).Scan(&patterns); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(patterns, []string{"orders.*"}) {
		t.Fatalf("installed patterns while a declaration waits = %v, want [orders.*]", patterns)
	}
	log, err = consumers.ListGroupBindingConfigLog(ctx, consumer.StreamId, consumer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 2 {
		t.Fatalf("binding log with a waiting declaration = %+v, want the installed and the waiting row", log)
	}

	// test: the instance's lease expires
	if _, err := groups.Datastore.Pool.Exec(ctx, "UPDATE "+instances+" SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	replaced, err := consumers.DeclareBindings(ctx, consumer.StreamId, consumer.Id, []string{"shipments.*"}, "host-b", time.Now())

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if replaced != consume.BindingInstalled {
		t.Fatalf("different set with no live instance = %s, want installed", replaced)
	}
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT array_agg(pattern ORDER BY pattern) FROM "+bindings+" WHERE consumer_group_id = $1", consumer.Id).Scan(&patterns); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(patterns, []string{"shipments.*"}) {
		t.Fatalf("installed patterns after the replacement = %v, want [shipments.*]", patterns)
	}
}

// invariant (cleanup): deleting a group removes its rows from the four
// per-stream tables with no foreign key to it -- claim lease, key lease,
// exception queue, delivery log -- and leaves a sibling group's rows alone.
func TestDeleteGroupRemovesItsUnlinkedRows(t *testing.T) {
	// setup: each group holds one row in each of the four tables
	groups, consumer := newMessageConsumerDatastore(t)
	sibling := registerConsumer(t, groups, consumer, "auditors")
	produceMessages(t, groups, consumer, 1)
	consumers := newConsumeDatastore(t, groups)
	exceptions := newExceptionConsumerDatastore(t, groups)
	keys := newKeyLeaseDatastore(t, groups)
	retry := (&common.RetryPolicy{}).WithDefaults()
	ctx := t.Context()
	for _, group := range []*consume.Consumer{consumer, sibling} {
		claimRange(t, groups, group)
		if _, err := keys.Claim(ctx, group.StreamId, group.Id, "order-1", 1, false, common.ConcurrencyExclusive, 0, 0, time.Minute, pgtype.UUID{Bytes: uuid.New(), Valid: true}); err != nil {
			t.Fatal(err)
		}
		insertException(t, groups, group, 1, "ready", -time.Hour, 0)
		claimed, err := exceptions.Claim(ctx, group.StreamId, group.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeFailures)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("exception Claim during setup = %+v, %v; want the one row", claimed, err)
		}
		if err := exceptions.RecordFailure(ctx, retry, &claimed[0], errors.New("handler returned an error"), stream.DeliveryLogModeFailures, nil); err != nil {
			t.Fatal(err)
		}
	}

	// test
	err := consumers.DeleteGroup(ctx, consumer.StreamId, consumer.Id, consumer.Name)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := consumers.GetGroup(ctx, consumer.StreamId, consumer.Name)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != nil {
		t.Fatalf("GetGroup(%s) after DeleteGroup = %+v, want nil", consumer.Name, deleted)
	}
	if got := unlinkedRows(t, groups, consumer); got != [4]int{0, 0, 0, 0} {
		t.Errorf("deleted group's rows in (claim lease, key lease, exception queue, delivery log) = %v, want [0 0 0 0]", got)
	}
	if got := unlinkedRows(t, groups, sibling); got != [4]int{1, 1, 1, 1} {
		t.Errorf("sibling group's rows in (claim lease, key lease, exception queue, delivery log) = %v, want [1 1 1 1]", got)
	}
}

// ***************
// *** HELPERS ***
// ***************

// unlinkedRows counts the group's rows in the four per-stream tables that
// carry no foreign key to consumer_group_config.
func unlinkedRows(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer) [4]int {
	t.Helper()
	schema := groups.Datastore.Schema + "."
	var counts [4]int
	if err := groups.Datastore.Pool.QueryRow(t.Context(),
		"SELECT (SELECT count(*) FROM "+schema+stream.ClaimLeaseTable(consumer.StreamId)+" WHERE consumer_group_id = $1),"+
			" (SELECT count(*) FROM "+schema+stream.MessageKeyLeaseTable(consumer.StreamId)+" WHERE consumer_group_id = $1),"+
			" (SELECT count(*) FROM "+schema+stream.ExceptionQueueTable(consumer.StreamId)+" WHERE consumer_group_id = $1),"+
			" (SELECT count(*) FROM "+schema+stream.DeliveryLogTable(consumer.StreamId)+" WHERE consumer_group_id = $1)",
		consumer.Id).Scan(&counts[0], &counts[1], &counts[2], &counts[3]); err != nil {
		t.Fatal(err)
	}
	return counts
}
