package metric

import (
	"testing"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric/controller/datastore"
)

// behavior: a worker snapshot resolves each worker's system, stream, and
// group columns through its owner chain, counts only unexpired instances as
// live, takes max_attempts over live instances alone, and reads
// unclaimed_for_secs as positive only once every instance has expired.
func TestWorkerSnapshotsResolveOwnersAndCountOnlyLiveInstances(t *testing.T) {
	// setup
	metrics, orders := newMetricDatastore(t)
	ctx := t.Context()
	workers := newWorkerDatastore(t, metrics)
	processor := registerConsumerGroup(t, metrics, orders, "processor")
	system, err := common.NewSystemOwner(orders.SystemId)
	if err != nil {
		t.Fatal(err)
	}
	streamOwner, err := common.NewStreamOwner(orders.SystemId, orders.Id, orders.Name)
	if err != nil {
		t.Fatal(err)
	}
	groupOwner, err := common.NewConsumerGroupOwner(orders.SystemId, orders.Id, processor.Id, processor.Name)
	if err != nil {
		t.Fatal(err)
	}
	declareWorker(t, workers, system, "manager")
	janitorId := declareWorker(t, workers, streamOwner, "janitor")
	consumerId := declareWorker(t, workers, groupOwner, "consumer")
	expired := claimInstance(t, workers, janitorId)
	recordFailures(t, workers, expired, 5)
	expireInstance(t, workers, expired.Id)
	live := claimInstance(t, workers, consumerId)
	recordFailures(t, workers, live, 2)
	stale := claimInstance(t, workers, consumerId)
	recordFailures(t, workers, stale, 7)
	expireInstance(t, workers, stale.Id)

	// test
	snapshots, err := metrics.WorkerSnapshots(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 3 {
		t.Fatalf("WorkerSnapshots() = %+v, want three rows", snapshots)
	}
	manager := workerSnapshot(t, snapshots, "manager")
	if manager.SystemId != orders.SystemId || manager.StreamId != 0 || manager.ConsumerGroupId != 0 || manager.StreamName != "" || manager.GroupName != "" {
		t.Errorf("WorkerSnapshots()[manager] = %+v, want the system id alone", manager)
	}
	if manager.LiveInstances != 0 || manager.MaxAttempts != 0 || manager.UnclaimedForSecs != 0 {
		t.Errorf("WorkerSnapshots()[manager] liveness = %+v, want no instances, 0 everywhere", manager)
	}
	janitor := workerSnapshot(t, snapshots, "janitor")
	if janitor.SystemId != orders.SystemId || janitor.StreamId != orders.Id || janitor.ConsumerGroupId != 0 || janitor.StreamName != "orders" || janitor.GroupName != "" {
		t.Errorf("WorkerSnapshots()[janitor] = %+v, want the system and stream ids with the stream name", janitor)
	}
	if janitor.LiveInstances != 0 || janitor.MaxAttempts != 0 || janitor.UnclaimedForSecs <= 0 {
		t.Errorf("WorkerSnapshots()[janitor] liveness = %+v, want no live instance, max attempts 0, unclaimed for a positive time", janitor)
	}
	consumer := workerSnapshot(t, snapshots, "consumer")
	if consumer.SystemId != orders.SystemId || consumer.StreamId != orders.Id || consumer.ConsumerGroupId != processor.Id || consumer.StreamName != "orders" || consumer.GroupName != "processor" {
		t.Errorf("WorkerSnapshots()[consumer] = %+v, want every owner id with the stream and group names", consumer)
	}
	if consumer.LiveInstances != 1 || consumer.MaxAttempts != 2 || consumer.UnclaimedForSecs > 0 {
		t.Errorf("WorkerSnapshots()[consumer] liveness = %+v, want one live instance, max attempts 2 from it alone, not unclaimed", consumer)
	}
}

// ***************
// *** HELPERS ***
// ***************

// workerSnapshot is the row named name.
func workerSnapshot(t testing.TB, snapshots []datastore.WorkerSnapshotRow, name string) datastore.WorkerSnapshotRow {
	t.Helper()
	for _, snapshot := range snapshots {
		if snapshot.Name == name {
			return snapshot
		}
	}
	t.Fatalf("WorkerSnapshots() = %+v, want a row named %s", snapshots, name)
	return datastore.WorkerSnapshotRow{}
}
