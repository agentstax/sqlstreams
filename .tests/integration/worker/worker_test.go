package worker

import (
	"reflect"
	"slices"
	"testing"

	"github.com/agentstax/sqlstreams/pkg/worker/controller/datastore"
	"golang.org/x/sync/errgroup"
)

// behavior: a redeclaration with different metadata writes it onto the
// existing row and appends one worker_config_log row; one with the same
// metadata writes nothing.
func TestRedeclareWritesMetadataAndOneLogRowOnlyWhenChanged(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)
	metadata := map[string]any{"rate": "1s"}
	ctx := t.Context()

	// test
	changedErr := workers.RegisterWorker(ctx, "collector", owner, metadata, 1, "worker_test")
	unchangedErr := workers.RegisterWorker(ctx, "collector", owner, metadata, 1, "worker_test")

	// verify
	if changedErr != nil || unchangedErr != nil {
		t.Fatalf("RegisterWorker twice = %v, %v; want nil, nil", changedErr, unchangedErr)
	}
	declared, err := workers.GetWorker(ctx, "collector", owner)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(declared.Metadata, metadata) {
		t.Fatalf("GetWorker().Metadata after redeclare = %+v, want %+v", declared.Metadata, metadata)
	}
	var logged int
	logs := workers.Datastore.Schema + ".worker_config_log"
	if err := workers.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE worker_id = $1", workerId).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 2 {
		t.Fatalf("worker_config_log rows after create, change, and unchanged redeclare = %d, want 2", logged)
	}
}

// behavior: target_instances is set at creation only, so a worker suspended
// at 0 stays suspended when a restarted process redeclares it.
func TestRedeclareKeepsTargetInstances(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	declareWorker(t, workers, owner, "collector", 0)
	ctx := t.Context()

	// test
	err := workers.RegisterWorker(ctx, "collector", owner, nil, 1, "worker_test")

	// verify
	if err != nil {
		t.Fatal(err)
	}
	declared, err := workers.GetWorker(ctx, "collector", owner)
	if err != nil {
		t.Fatal(err)
	}
	if declared.TargetInstances != 0 {
		t.Fatalf("GetWorker().TargetInstances after redeclare at 1 = %d, want 0", declared.TargetInstances)
	}
}

// invariant (idempotent declaration): concurrent first declarations of one
// (name, owner) all succeed and leave one worker row with one log row.
func TestConcurrentFirstDeclarationsLeaveOneRow(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	ctx := t.Context()

	// test
	var group errgroup.Group
	for range 8 {
		group.Go(func() error {
			return workers.RegisterWorker(ctx, "collector", owner, nil, 1, "worker_test")
		})
	}
	err := group.Wait()

	// verify
	if err != nil {
		t.Fatalf("RegisterWorker from 8 concurrent declarers = %v, want nil", err)
	}
	var rows int
	var logged int
	configs := workers.Datastore.Schema + ".worker_config"
	logs := workers.Datastore.Schema + ".worker_config_log"
	if err := workers.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+configs+" WHERE name = 'collector'").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if err := workers.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE name = 'collector'").Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || logged != 1 {
		t.Fatalf("worker_config rows, worker_config_log rows after 8 concurrent declarations = %d, %d; want 1, 1", rows, logged)
	}
}

// behavior: ListWorkers follows the owner chain downward -- the system sees
// every row, a stream sees its own and its groups', a group sees only its
// own -- and never a sibling's; a row's owner columns resolve through the
// joins.
func TestListWorkersFollowsTheOwnerChain(t *testing.T) {
	// setup
	workers, system := newWorkerDatastore(t)
	orders := declareStreamOwner(t, workers, system, "orders")
	billing := declareConsumerGroupOwner(t, workers, orders, "billing")
	shipments := declareStreamOwner(t, workers, system, "shipments")
	systemWorker := declareWorker(t, workers, system, "manager", 1)
	ordersWorker := declareWorker(t, workers, orders, "janitor", 1)
	billingWorker := declareWorker(t, workers, billing, "consumer", 1)
	shipmentsWorker := declareWorker(t, workers, shipments, "janitor", 1)
	ctx := t.Context()

	// test
	bySystem, systemErr := workers.ListWorkers(ctx, system)
	byOrders, ordersErr := workers.ListWorkers(ctx, orders)
	byBilling, billingErr := workers.ListWorkers(ctx, billing)
	byShipments, shipmentsErr := workers.ListWorkers(ctx, shipments)

	// verify
	if systemErr != nil || ordersErr != nil || billingErr != nil || shipmentsErr != nil {
		t.Fatalf("ListWorkers by system, stream, group, sibling stream = %v, %v, %v, %v; want nil", systemErr, ordersErr, billingErr, shipmentsErr)
	}
	if got := workerIds(bySystem); !reflect.DeepEqual(got, []int64{systemWorker, ordersWorker, billingWorker, shipmentsWorker}) {
		t.Errorf("ListWorkers(system) = %v, want every row %v", got, []int64{systemWorker, ordersWorker, billingWorker, shipmentsWorker})
	}
	if got := workerIds(byOrders); !reflect.DeepEqual(got, []int64{ordersWorker, billingWorker}) {
		t.Errorf("ListWorkers(orders) = %v, want its own and its group's %v", got, []int64{ordersWorker, billingWorker})
	}
	if got := workerIds(byShipments); !reflect.DeepEqual(got, []int64{shipmentsWorker}) {
		t.Errorf("ListWorkers(shipments) = %v, want only its own %v", got, []int64{shipmentsWorker})
	}
	if got := workerIds(byBilling); !reflect.DeepEqual(got, []int64{billingWorker}) {
		t.Fatalf("ListWorkers(billing) = %v, want only its own %v", got, []int64{billingWorker})
	}
	row := byBilling[0]
	if row.OwnerSystemId != system.SystemId || row.OwnerStreamId != orders.StreamId || row.StreamName != "orders" || row.ConsumerGroup != "billing" {
		t.Errorf("group-owned row's owner columns = (system %d, stream %d %q, group %q), want (%d, %d \"orders\", \"billing\")", row.OwnerSystemId, row.OwnerStreamId, row.StreamName, row.ConsumerGroup, system.SystemId, orders.StreamId)
	}
}

// ***************
// *** HELPERS ***
// ***************

// workerIds is the rows' ids in ascending order.
func workerIds(rows []datastore.ListWorkersRow) []int64 {
	var ids []int64
	for _, row := range rows {
		ids = append(ids, row.Id)
	}
	slices.Sort(ids)
	return ids
}
