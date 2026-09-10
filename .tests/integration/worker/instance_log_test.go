package worker

import (
	"testing"
	"time"
	"uuid"
)

// behavior: the snapshot window keeps every log row whose lease was still
// live at the window's start; a renewal is its own row beside the claim's.
func TestInstanceSnapshotsWindowDropsLeasesExpiredBeforeIt(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", -1)
	expired := claimInstance(t, workers, workerId)
	expireInstance(t, workers, expired.Id)
	live := claimInstance(t, workers, workerId)
	ctx := t.Context()
	if err := workers.RenewInstance(ctx, live.Id, uuid.UUID(live.Token.Bytes), time.Hour); err != nil {
		t.Fatal(err)
	}

	// test: a window opening now, wide enough that the server's clock cannot fall outside it
	snapshots, err := workers.ListInstanceSnapshots(ctx, workerId, time.Now(), time.Now().Add(time.Minute))

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("ListInstanceSnapshots(now, now+1m) = %d rows %+v, want 2", len(snapshots), snapshots)
	}
	if snapshots[0].WorkerInstanceId != live.Id || snapshots[1].WorkerInstanceId != live.Id {
		t.Fatalf("ListInstanceSnapshots(now, now+1m) instance ids = %d, %d; want %d, %d", snapshots[0].WorkerInstanceId, snapshots[1].WorkerInstanceId, live.Id, live.Id)
	}
	if snapshots[0].ExpiresAt.Equal(snapshots[1].ExpiresAt) {
		t.Fatalf("ListInstanceSnapshots(now, now+1m) expiries = %v, %v; want the renewal's later than the claim's", snapshots[0].ExpiresAt, snapshots[1].ExpiresAt)
	}
}

// behavior: the log sweep keeps a row until its lease has been expired for
// ttl, however old the row is; a live lease's row is never swept.
func TestInstanceLogSweepRetainsByLeaseExpiryPlusTtl(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", -1)
	expired := claimInstance(t, workers, workerId)
	expireInstance(t, workers, expired.Id)
	live := claimInstance(t, workers, workerId)
	ctx := t.Context()

	// test
	withinTtl, withinErr := workers.SweepExpiredInstanceLogs(ctx, time.Hour)
	pastTtl, pastErr := workers.SweepExpiredInstanceLogs(ctx, 0)

	// verify
	if withinErr != nil || pastErr != nil {
		t.Fatalf("SweepExpiredInstanceLogs twice = %v, %v; want nil, nil", withinErr, pastErr)
	}
	if withinTtl != 0 || pastTtl != 1 {
		t.Fatalf("SweepExpiredInstanceLogs(1h), then (0) = %d, %d; want 0, 1", withinTtl, pastTtl)
	}
	var remaining int64
	logs := workers.Datastore.Schema + ".worker_instance_log"
	if err := workers.Datastore.Pool.QueryRow(ctx, "SELECT min(worker_instance_id) FROM "+logs+" WHERE worker_id = $1", workerId).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != live.Id {
		t.Fatalf("worker_instance_log row left after the sweeps = instance %d, want the live instance %d", remaining, live.Id)
	}
}
