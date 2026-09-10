package worker

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/.tests/integration/postgres"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	systemcontroller "github.com/agentstax/sqlstreams/pkg/system/controller"
	"github.com/agentstax/sqlstreams/pkg/worker/controller/datastore"
)

// newWorkerDatastore registers a system, which creates the worker tables,
// and returns the worker datastore with the system's owner.
func newWorkerDatastore(t testing.TB) (*datastore.WorkerDatastore, *common.Owner) {
	t.Helper()
	ds := postgres.Start(t)
	systems, err := systemcontroller.NewSystemController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := systems.Register(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	owner, err := common.NewSystemOwner(registered.Id)
	if err != nil {
		t.Fatal(err)
	}
	workers, err := datastore.NewWorkerDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return workers, owner
}

// declareWorker registers one worker row and returns its id.
func declareWorker(t testing.TB, workers *datastore.WorkerDatastore, owner *common.Owner, name string, targetInstances int) int64 {
	t.Helper()
	if err := workers.RegisterWorker(t.Context(), name, owner, nil, targetInstances, "worker_test"); err != nil {
		t.Fatal(err)
	}
	declared, err := workers.GetWorker(t.Context(), name, owner)
	if err != nil {
		t.Fatal(err)
	}
	return declared.Id
}

// claimInstance claims one instance of the worker and returns its row.
func claimInstance(t testing.TB, workers *datastore.WorkerDatastore, workerId int64) *datastore.WorkerInstanceRow {
	t.Helper()
	instance, err := workers.ClaimInstance(t.Context(), workerId, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if instance == nil {
		t.Fatalf("ClaimInstance(%d) declined during setup", workerId)
	}
	return instance
}

// expireInstance moves the instance's lease expiry into the past, on the
// row and on its worker_instance_log copies.
func expireInstance(t testing.TB, workers *datastore.WorkerDatastore, instanceId int64) {
	t.Helper()
	instances := workers.Datastore.Schema + ".worker_instance"
	logs := workers.Datastore.Schema + ".worker_instance_log"
	if _, err := workers.Datastore.Pool.Exec(t.Context(), "UPDATE "+instances+" SET expires_at = now() - interval '1 second' WHERE id = $1", instanceId); err != nil {
		t.Fatal(err)
	}
	if _, err := workers.Datastore.Pool.Exec(t.Context(), "UPDATE "+logs+" SET expires_at = now() - interval '1 second' WHERE worker_instance_id = $1", instanceId); err != nil {
		t.Fatal(err)
	}
}

// rejectInstanceLogInserts makes every new worker_instance_log row fail;
// NOT VALID leaves the rows already there alone.
func rejectInstanceLogInserts(t testing.TB, workers *datastore.WorkerDatastore) {
	t.Helper()
	table := workers.Datastore.Schema + ".worker_instance_log"
	if _, err := workers.Datastore.Pool.Exec(t.Context(), "ALTER TABLE "+table+" ADD CONSTRAINT reject_inserts CHECK (false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
}

// declareStreamOwner registers a stream under the system and returns its
// owner.
func declareStreamOwner(t testing.TB, workers *datastore.WorkerDatastore, system *common.Owner, name string) *common.Owner {
	t.Helper()
	streams, err := streamcontroller.NewStreamController(workers.Datastore, workers.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := streams.Register(t.Context(), system.SystemId, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := common.NewStreamOwner(registered.SystemId, registered.Id, registered.Name)
	if err != nil {
		t.Fatal(err)
	}
	return owner
}

// declareConsumerGroupOwner registers a consumer group on the stream and
// returns its owner.
func declareConsumerGroupOwner(t testing.TB, workers *datastore.WorkerDatastore, stream *common.Owner, name string) *common.Owner {
	t.Helper()
	groups, err := consumecontroller.NewConsumeController(workers.Datastore, workers.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := groups.RegisterGroup(t.Context(), stream.StreamId, name, consume.CursorPosition{})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := common.NewConsumerGroupOwner(stream.SystemId, stream.StreamId, registered.Id, registered.Name)
	if err != nil {
		t.Fatal(err)
	}
	return owner
}
