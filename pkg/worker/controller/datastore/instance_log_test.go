package datastore_test

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/sqlstreamstest"
	systemcontroller "github.com/agentstax/sqlstreams/pkg/system/controller"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workerdatastore "github.com/agentstax/sqlstreams/pkg/worker/controller/datastore"
	"github.com/jackc/pgx/v5/pgxpool"
)

// instanceLogTest is one registered system with one declared worker, and
// the qualified table names the narratives read and alter directly.
type instanceLogTest struct {
	pool        *pgxpool.Pool
	systems     *systemcontroller.SystemController
	workers     *workerdatastore.WorkerDatastore
	workerId    int64
	instances   string
	instanceLog string
}

// invariant (crash consistency): every claim and every renewal appends
// exactly one worker_instance_log row, and the newest row is the live row's
// image.
func TestClaimAndRenewalEachAppendOneSnapshot(t *testing.T) {
	test := newInstanceLogTest(t)
	ctx := t.Context()

	claimed, err := test.workers.ClaimInstance(ctx, test.workerId, time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("ClaimInstance() = %+v, %v; want a row, nil", claimed, err)
	}
	if got := test.snapshotCount(t); got != 1 {
		t.Fatalf("snapshots after claim = %d, want 1", got)
	}

	declined, err := test.workers.ClaimInstance(ctx, test.workerId, time.Minute)
	if err != nil || declined != nil {
		t.Fatalf("ClaimInstance() at target = %+v, %v; want nil, nil", declined, err)
	}
	if got := test.snapshotCount(t); got != 1 {
		t.Fatalf("snapshots after a declined claim = %d, want 1", got)
	}

	token := uuid.UUID(claimed.Token.Bytes)
	if _, err := test.workers.RecordInstanceFailure(ctx, claimed.Id, token); err != nil {
		t.Fatal(err)
	}
	if err := test.workers.RenewInstance(ctx, claimed.Id, token, 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	if got := test.snapshotCount(t); got != 2 {
		t.Fatalf("snapshots after renewal = %d, want 2", got)
	}

	var matches bool
	err = test.pool.QueryRow(ctx, `
		SELECT log.worker_id = live.worker_id
			AND log.token = live.token
			AND log.expires_at = live.expires_at
			AND log.attempts = live.attempts
			AND log.created_at = live.created_at
		FROM `+test.instanceLog+` log
		JOIN `+test.instances+` live ON live.id = log.worker_instance_id
		ORDER BY log.id DESC LIMIT 1;
	`).Scan(&matches)
	if err != nil {
		t.Fatal(err)
	}
	if !matches {
		t.Fatal("newest snapshot differs from the live worker_instance row, want the same image")
	}
}

// invariant: a renewal the instance has lost -- wrong token, expired lease
// -- returns ErrInstanceLost and writes nothing; release and expiry sweeps
// remove the live row but never its history.
func TestLostRenewalsAndSweepsLeaveHistoryUnchanged(t *testing.T) {
	test := newInstanceLogTest(t)
	ctx := t.Context()
	claimed, err := test.workers.ClaimInstance(ctx, test.workerId, time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("ClaimInstance() = %+v, %v; want a row, nil", claimed, err)
	}
	token := uuid.UUID(claimed.Token.Bytes)

	err = test.workers.RenewInstance(ctx, claimed.Id, uuid.NewV7(), time.Minute)
	if !errors.Is(err, worker.ErrInstanceLost) {
		t.Fatalf("RenewInstance(wrong token) = %v, want ErrInstanceLost", err)
	}
	if got := test.snapshotCount(t); got != 1 {
		t.Fatalf("snapshots after a wrong-token renewal = %d, want 1", got)
	}

	if err := test.workers.ReleaseInstance(ctx, claimed.Id, token); err != nil {
		t.Fatal(err)
	}
	if got := test.snapshotCount(t); got != 1 {
		t.Fatalf("snapshots after release = %d, want 1", got)
	}

	claimed, err = test.workers.ClaimInstance(ctx, test.workerId, time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("ClaimInstance() after release = %+v, %v; want a row, nil", claimed, err)
	}
	if _, err := test.pool.Exec(ctx, "UPDATE "+test.instances+" SET expires_at = now() - interval '1 second' WHERE id = $1", claimed.Id); err != nil {
		t.Fatal(err)
	}
	err = test.workers.RenewInstance(ctx, claimed.Id, uuid.UUID(claimed.Token.Bytes), time.Minute)
	if !errors.Is(err, worker.ErrInstanceLost) {
		t.Fatalf("RenewInstance(expired lease) = %v, want ErrInstanceLost", err)
	}
	removed, err := test.workers.SweepExpiredInstances(ctx)
	if err != nil || removed != 1 {
		t.Fatalf("SweepExpiredInstances() = %d, %v; want 1, nil", removed, err)
	}
	if got := test.snapshotCount(t); got != 2 {
		t.Fatalf("snapshots after an expired renewal and sweep = %d, want 2", got)
	}
}

// invariant (crash consistency): a claim or renewal whose log write is
// rejected does not commit -- the live row is untouched or absent.
func TestClaimAndRenewalCannotCommitWithoutSnapshot(t *testing.T) {
	test := newInstanceLogTest(t)
	ctx := t.Context()
	claimed, err := test.workers.ClaimInstance(ctx, test.workerId, time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("ClaimInstance() = %+v, %v; want a row, nil", claimed, err)
	}
	token := uuid.UUID(claimed.Token.Bytes)

	// every log insert now fails its CHECK; NOT VALID leaves the existing row
	if _, err := test.pool.Exec(ctx, "ALTER TABLE "+test.instanceLog+" ADD CONSTRAINT reject_log CHECK (false) NOT VALID"); err != nil {
		t.Fatal(err)
	}

	if err := test.workers.RenewInstance(ctx, claimed.Id, token, time.Hour); err == nil {
		t.Fatal("RenewInstance() with the log write rejected = nil, want error")
	}
	var unchanged bool
	err = test.pool.QueryRow(ctx, `
		SELECT live.expires_at = log.expires_at
		FROM `+test.instances+` live
		JOIN `+test.instanceLog+` log ON log.worker_instance_id = live.id
		ORDER BY log.id DESC LIMIT 1;
	`).Scan(&unchanged)
	if err != nil {
		t.Fatal(err)
	}
	if !unchanged {
		t.Fatal("live expires_at moved after a rejected renewal, want the last snapshot's value")
	}

	if err := test.workers.ReleaseInstance(ctx, claimed.Id, token); err != nil {
		t.Fatal(err)
	}
	if _, err := test.workers.ClaimInstance(ctx, test.workerId, time.Minute); err == nil {
		t.Fatal("ClaimInstance() with the log write rejected = nil, want error")
	}
	var live int
	if err := test.pool.QueryRow(ctx, "SELECT count(*) FROM "+test.instances).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Fatalf("live instances after a rejected claim = %d, want 0", live)
	}
}

// behavior: ListInstanceSnapshots returns the snapshots whose lease covers
// the window, both bounds inclusive, longest lease first at a shared start,
// and never another worker's.
func TestInstanceSnapshotsReadByInclusiveLeaseWindow(t *testing.T) {
	test := newInstanceLogTest(t)
	ctx := t.Context()
	claimed, err := test.workers.ClaimInstance(ctx, test.workerId, time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("ClaimInstance() = %+v, %v; want a row, nil", claimed, err)
	}
	if err := test.workers.RenewInstance(ctx, claimed.Id, uuid.UUID(claimed.Token.Bytes), 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	current, err := test.workers.CurrentTime(ctx)
	if err != nil {
		t.Fatal(err)
	}

	snapshots, err := test.workers.ListInstanceSnapshots(ctx, test.workerId, current, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("ListInstanceSnapshots(now, now) = %d snapshots, want 2", len(snapshots))
	}
	if !snapshots[0].ExpiresAt.After(snapshots[1].ExpiresAt) {
		t.Errorf("ListInstanceSnapshots(now, now) order = expiries %v then %v, want the longest lease first", snapshots[0].ExpiresAt, snapshots[1].ExpiresAt)
	}
	for _, snapshot := range snapshots {
		if snapshot.WorkerInstanceId != claimed.Id || snapshot.WorkerId != test.workerId || snapshot.Token != claimed.Token || snapshot.AttemptedAt.IsZero() {
			t.Errorf("ListInstanceSnapshots(now, now) row = %+v, want instance %d of worker %d with its token and attempted_at", snapshot, claimed.Id, test.workerId)
		}
	}

	claimedAt := snapshots[0].CreatedAt
	atClaim, err := test.workers.ListInstanceSnapshots(ctx, test.workerId, claimedAt, claimedAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(atClaim) != 1 {
		t.Errorf("ListInstanceSnapshots(claimed_at, claimed_at) = %d snapshots, want 1: the renewal starts later", len(atClaim))
	}
	expiresAt := snapshots[0].ExpiresAt
	atExpiry, err := test.workers.ListInstanceSnapshots(ctx, test.workerId, expiresAt, expiresAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(atExpiry) != 1 {
		t.Errorf("ListInstanceSnapshots(expires_at, expires_at) = %d snapshots, want 1: the expiry bound is inclusive", len(atExpiry))
	}
	otherWorker, err := test.workers.ListInstanceSnapshots(ctx, test.workerId+1, current, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherWorker) != 0 {
		t.Errorf("ListInstanceSnapshots(other worker) = %d snapshots, want 0", len(otherWorker))
	}
}

// invariant: retention drops a snapshot by its lease expiry, never by its
// record time alone, so old rows still covering a recent or live lease stay.
func TestSnapshotRetentionKeepsLiveLeaseCoverage(t *testing.T) {
	test := newInstanceLogTest(t)
	ctx := t.Context()
	claimed, err := test.workers.ClaimInstance(ctx, test.workerId, time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("ClaimInstance() = %+v, %v; want a row, nil", claimed, err)
	}

	// three copies of the claim's snapshot recorded two days ago, with leases
	// that expired 25 hours ago, 23 hours ago, and one hour from now
	for _, expiryHours := range []int{-25, -23, 1} {
		_, err := test.pool.Exec(ctx, `
			INSERT INTO `+test.instanceLog+`
				(worker_instance_id, worker_id, token, expires_at, attempts, created_at, attempted_at)
			SELECT
				worker_instance_id,
				worker_id,
				token,
				now() + make_interval(hours => $1),
				attempts,
				now() - interval '48 hours',
				now() - interval '48 hours'
			FROM `+test.instanceLog+`
			ORDER BY id LIMIT 1;
		`, expiryHours)
		if err != nil {
			t.Fatal(err)
		}
	}

	removed, err := test.workers.SweepExpiredInstanceLogs(ctx, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("SweepExpiredInstanceLogs(24h) = %d removed, want 1: only the lease expired beyond the ttl", removed)
	}
	if got := test.snapshotCount(t); got != 3 {
		t.Errorf("snapshots after retention = %d, want 3", got)
	}
}

// behavior: deleting the system drops its instance history with it.
func TestSystemDeleteDropsInstanceLog(t *testing.T) {
	test := newInstanceLogTest(t)
	ctx := t.Context()

	if err := test.systems.Delete(ctx); err != nil {
		t.Fatal(err)
	}

	var dropped bool
	if err := test.pool.QueryRow(ctx, "SELECT to_regclass($1) IS NULL", test.instanceLog).Scan(&dropped); err != nil {
		t.Fatal(err)
	}
	if !dropped {
		t.Fatalf("to_regclass(%q) after Delete() is not null, want the table dropped", test.instanceLog)
	}
}

// snapshotCount reads the worker_instance_log row count; it fails only when
// the read itself does.
func (i *instanceLogTest) snapshotCount(t testing.TB) int {
	t.Helper()
	var count int
	if err := i.pool.QueryRow(t.Context(), "SELECT count(*) FROM "+i.instanceLog).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// ***************
// *** HELPERS ***
// ***************

// newInstanceLogTest registers a system through its controller, so the
// tables are the registry's own, and declares one worker with one instance
// allowed.
func newInstanceLogTest(t testing.TB) *instanceLogTest {
	t.Helper()
	ds := sqlstreamstest.NewDatastore(t, nil)
	ctx := t.Context()
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
	return &instanceLogTest{
		pool:        ds.Pool,
		systems:     systems,
		workers:     workers,
		workerId:    declared.Id,
		instances:   ds.Schema + ".worker_instance",
		instanceLog: ds.Schema + ".worker_instance_log",
	}
}
