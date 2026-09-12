package worker

import (
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
)

func TestOperationalTargetSurvivesRegistrationAndIsAudited(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	id := declareWorker(t, workers, owner, "vacuum", 0)
	ctx := t.Context()

	// test
	err := workers.UpdateTargetInstances(ctx, id, 1)

	// verify
	if err != nil {
		t.Fatal(err)
	}

	// test
	err = workers.RegisterWorker(ctx, "vacuum", owner, nil, 0, "worker_test")

	// verify
	if err != nil {
		t.Fatal(err)
	}
	current, err := workers.GetWorker(ctx, "vacuum", owner)
	if err != nil {
		t.Fatal(err)
	}
	if current.TargetInstances != 1 {
		t.Fatalf("RegisterWorker(initial 0) target = %d, want 1", current.TargetInstances)
	}

	// test
	err = workers.UpdateTargetInstances(ctx, id, 0)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	var targets []int
	if err := workers.Datastore.Pool.QueryRow(ctx, "SELECT array_agg(target_instances ORDER BY id) FROM "+workers.Datastore.Schema+".worker_config_log WHERE worker_id=$1", id).Scan(&targets); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(targets, []int{0, 1, 0}) {
		t.Errorf("UpdateTargetInstances history = %v, want [0 1 0]", targets)
	}
}

func TestSuspensionStopsRenewalWithoutReleasingAClaimEarly(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	id := declareWorker(t, workers, owner, "vacuum", 1)
	held := claimInstance(t, workers, id)
	ctx := t.Context()
	token := uuid.UUID(held.Token.Bytes)
	if err := workers.UpdateTargetInstances(ctx, id, 0); err != nil {
		t.Fatal(err)
	}

	// test
	err := workers.RenewInstance(ctx, held.Id, token, time.Minute)

	// verify
	if !errors.Is(err, worker.ErrWorkerSuspended) {
		t.Fatalf("RenewInstance(suspended) = %v, want ErrWorkerSuspended", err)
	}

	// test
	err = workers.UpdateTargetInstances(ctx, id, 1)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	second, err := workers.ClaimInstance(ctx, id, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if second != nil {
		t.Fatal("ClaimInstance(before previous release) claimed, want declined")
	}

	// test
	err = workers.ReleaseInstance(ctx, held.Id, token)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	second, err = workers.ClaimInstance(ctx, id, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if second == nil {
		t.Fatal("ClaimInstance(after release) declined, want claimed")
	}
}

func TestTargetChangeRollsBackWhenHistoryCannotBeWritten(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	id := declareWorker(t, workers, owner, "vacuum", 0)
	ctx := t.Context()
	if _, err := workers.Datastore.Pool.Exec(ctx, "ALTER TABLE "+workers.Datastore.Schema+".worker_config_log ADD CONSTRAINT reject_target_change CHECK (target_instances=0) NOT VALID"); err != nil {
		t.Fatal(err)
	}

	// test
	err := workers.UpdateTargetInstances(ctx, id, 1)

	// verify
	if err == nil {
		t.Fatal("UpdateTargetInstances(rejected history) = nil, want error")
	}
	current, err := workers.GetWorker(ctx, "vacuum", owner)
	if err != nil {
		t.Fatal(err)
	}
	if current.TargetInstances != 0 {
		t.Errorf("UpdateTargetInstances(rejected history) target = %d, want 0", current.TargetInstances)
	}
}
