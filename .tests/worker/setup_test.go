package worker

import (
	"testing"

	"github.com/agentstax/sqlstreams/.tests/postgres"
	"github.com/agentstax/sqlstreams/pkg/common"
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
