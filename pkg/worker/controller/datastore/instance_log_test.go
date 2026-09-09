package datastore_test

import (
	"errors"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/sqlstreamstest"
	systemcontroller "github.com/agentstax/sqlstreams/pkg/system/controller"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workerdatastore "github.com/agentstax/sqlstreams/pkg/worker/controller/datastore"
)

func TestWorkerInstanceLog(t *testing.T) {
	ds := sqlstreamstest.NewDatastore(t, nil)
	ctx := t.Context()
	pool := ds.Pool
	schema := ds.Schema
	systems, err := systemcontroller.NewSystemController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := systems.Register(ctx)
	if err != nil {
		t.Fatal(err)
	}
	workers, err := workerdatastore.NewWorkerDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := common.NewSystemOwner(registered.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := workers.RegisterWorker(ctx, "manager", owner, nil, 1, "instance_log_test"); err != nil {
		t.Fatal(err)
	}
	declared, err := workers.GetWorker(ctx, "manager", owner)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := workers.ClaimInstance(ctx, declared.Id, time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim = %+v, error = %v", claimed, err)
	}
	token := uuid.UUID(claimed.Token.Bytes)
	countLogs := func() int {
		t.Helper()
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".worker_instance_log").Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	if countLogs() != 1 {
		t.Fatal("claim did not append one snapshot")
	}
	declined, err := workers.ClaimInstance(ctx, declared.Id, time.Minute)
	if err != nil || declined != nil || countLogs() != 1 {
		t.Fatalf("declined claim changed history: %+v, %v", declined, err)
	}
	if err := workers.RenewInstance(ctx, claimed.Id, uuid.NewV7(), time.Minute); !errors.Is(err, worker.ErrInstanceLost) || countLogs() != 1 {
		t.Fatalf("wrong-token renewal changed history: %v", err)
	}
	if _, err := workers.RecordInstanceFailure(ctx, claimed.Id, token); err != nil {
		t.Fatal(err)
	}
	if err := workers.RenewInstance(ctx, claimed.Id, token, 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	if countLogs() != 2 {
		t.Fatal("renewal did not append one snapshot")
	}
	current, err := workers.CurrentTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := workers.ListInstanceSnapshots(ctx, declared.Id, current, current)
	if err != nil || len(snapshots) != 2 {
		t.Fatalf("boundary-crossing snapshots = %+v, error = %v", snapshots, err)
	}
	if !snapshots[0].ExpiresAt.After(snapshots[1].ExpiresAt) {
		t.Fatal("same-start snapshots must put the longest lease first")
	}
	for _, snapshot := range snapshots {
		if snapshot.Id <= 0 || snapshot.WorkerInstanceId != claimed.Id || snapshot.WorkerId != declared.Id || snapshot.Token != claimed.Token || snapshot.AttemptedAt.IsZero() {
			t.Fatalf("snapshot did not retain the full log row: %+v", snapshot)
		}
	}
	if snapshots[0].Attempts != 1 || snapshots[1].Attempts != 0 {
		t.Fatalf("snapshot attempts = %d, %d; want 1, 0", snapshots[0].Attempts, snapshots[1].Attempts)
	}
	firstCreatedAt := snapshots[0].CreatedAt
	atClaim, err := workers.ListInstanceSnapshots(ctx, declared.Id, firstCreatedAt, firstCreatedAt)
	if err != nil || len(atClaim) != 1 {
		t.Fatalf("read included a later renewal: %+v, %v", atClaim, err)
	}
	atExpiry, err := workers.ListInstanceSnapshots(ctx, declared.Id, snapshots[0].ExpiresAt, snapshots[0].ExpiresAt)
	if err != nil || len(atExpiry) != 1 {
		t.Fatalf("read excluded the inclusive expiry boundary: %+v, %v", atExpiry, err)
	}
	otherWorker, err := workers.ListInstanceSnapshots(ctx, declared.Id+1, current, current)
	if err != nil || len(otherWorker) != 0 {
		t.Fatalf("read included another worker: %+v, %v", otherWorker, err)
	}
	var matches bool
	err = pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT log.worker_id = live.worker_id
			AND log.token = live.token
			AND log.expires_at = live.expires_at
			AND log.attempts = live.attempts
			AND log.created_at = live.created_at
		FROM %s.worker_instance_log log
		JOIN %s.worker_instance live ON live.id = log.worker_instance_id
		ORDER BY log.id DESC LIMIT 1;
	`, schema, schema)).Scan(&matches)
	if err != nil || !matches {
		t.Fatalf("snapshot differs from source: %v", err)
	}

	// Reject log writes to prove renewal and claim cannot commit without history.
	if _, err := pool.Exec(ctx, "ALTER TABLE "+schema+".worker_instance_log ADD CONSTRAINT reject_log CHECK (false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
	if err := workers.RenewInstance(ctx, claimed.Id, token, time.Hour); err == nil {
		t.Fatal("renewal succeeded without a log write")
	}
	var unchanged bool
	err = pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT live.expires_at = log.expires_at
		FROM %s.worker_instance live
		JOIN %s.worker_instance_log log ON log.worker_instance_id = live.id
		ORDER BY log.id DESC LIMIT 1;
	`, schema, schema)).Scan(&unchanged)
	if err != nil || !unchanged || countLogs() != 2 {
		t.Fatalf("failed log write changed renewal: %v", err)
	}
	if err := workers.ReleaseInstance(ctx, claimed.Id, token); err != nil {
		t.Fatal(err)
	}
	if countLogs() != 2 {
		t.Fatal("release removed history")
	}
	if _, err := workers.ClaimInstance(ctx, declared.Id, time.Minute); err == nil {
		t.Fatal("claim succeeded without a log write")
	}
	var liveCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+schema+".worker_instance").Scan(&liveCount); err != nil || liveCount != 0 {
		t.Fatalf("failed log write left a claimed instance: %d, %v", liveCount, err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+schema+".worker_instance_log DROP CONSTRAINT reject_log"); err != nil {
		t.Fatal(err)
	}

	// Old record times alone must not discard recent or still-live lease coverage.
	for _, expiryHours := range []int{-25, -23, 1} {
		_, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.worker_instance_log
				(worker_instance_id, worker_id, token, expires_at, attempts, created_at, attempted_at)
			SELECT
				worker_instance_id,
				worker_id,
				token,
				now() + make_interval(hours => $1),
				attempts,
				now() - interval '48 hours',
				now() - interval '48 hours'
			FROM %s.worker_instance_log
			ORDER BY id LIMIT 1;
		`, schema, schema), expiryHours)
		if err != nil {
			t.Fatal(err)
		}
	}
	removed, err := workers.SweepExpiredInstanceLogs(ctx, 24*time.Hour)
	if err != nil || removed != 1 || countLogs() != 4 {
		t.Fatalf("retention removed %d snapshots, error = %v", removed, err)
	}

	claimed, err = workers.ClaimInstance(ctx, declared.Id, time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("second claim = %+v, error = %v", claimed, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE "+schema+".worker_instance SET expires_at = now() - interval '1 second' WHERE id = $1", claimed.Id); err != nil {
		t.Fatal(err)
	}
	if err := workers.RenewInstance(ctx, claimed.Id, uuid.UUID(claimed.Token.Bytes), time.Minute); !errors.Is(err, worker.ErrInstanceLost) || countLogs() != 5 {
		t.Fatalf("expired renewal changed history: %v", err)
	}
	removed, err = workers.SweepExpiredInstances(ctx)
	if err != nil || removed != 1 || countLogs() != 5 {
		t.Fatalf("instance cleanup removed history: %d, %v", removed, err)
	}
	if err := systems.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	var logRemoved bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NULL", schema+".worker_instance_log").Scan(&logRemoved); err != nil || !logRemoved {
		t.Fatalf("system deletion left instance history: %v", err)
	}
}
