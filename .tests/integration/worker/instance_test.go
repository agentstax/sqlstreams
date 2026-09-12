package worker

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/controller/datastore"
	"golang.org/x/sync/errgroup"
)

// invariant (one live lease): concurrent claims against a target of one
// insert exactly one worker_instance row.
func TestConcurrentClaimsAtTargetOneYieldOneInstance(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)

	// test
	claims := make(chan *datastore.WorkerInstanceRow, 16)
	var group errgroup.Group
	for range 16 {
		group.Go(func() error {
			instance, err := workers.ClaimInstance(t.Context(), workerId, time.Minute)
			claims <- instance
			return err
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
	close(claims)

	// verify
	claimed := 0
	for instance := range claims {
		if instance != nil {
			claimed++
		}
	}
	if claimed != 1 {
		t.Fatalf("rows claimed by 16 concurrent claimants = %d, want 1", claimed)
	}
}

func TestDeclinedClaimDoesNotWaitOnTheWorkerRowLock(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)
	claimInstance(t, workers, workerId)
	lockWorkerRow(t, workers, workerId)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// test
	instance, err := workers.ClaimInstance(ctx, workerId, time.Minute)

	// verify
	if err != nil {
		t.Fatalf("ClaimInstance(%d) at a full target while the worker row is locked = %v, want declined", workerId, err)
	}
	if instance != nil {
		t.Fatalf("ClaimInstance(%d) at a full target = %+v, want nil", workerId, instance)
	}
}

// behavior: a worker whose target_instances is 0 is suspended -- every
// claim is declined.
func TestClaimDeclinesAtTargetZero(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 0)

	// test
	instance, err := workers.ClaimInstance(t.Context(), workerId, time.Minute)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if instance != nil {
		t.Fatalf("ClaimInstance(%d) at target 0 = %+v, want nil", workerId, instance)
	}
}

// behavior: a target_instances of -1 is no cap -- every claim inserts a row
// no matter how many are live.
func TestClaimAlwaysSucceedsAtUnboundedTarget(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", -1)
	claimInstance(t, workers, workerId)
	claimInstance(t, workers, workerId)

	// test
	instance, err := workers.ClaimInstance(t.Context(), workerId, time.Minute)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if instance == nil {
		t.Fatalf("ClaimInstance(%d) at target -1 with two live instances = nil, want a row", workerId)
	}
}

// behavior: a claim against a worker id no row holds is declined, not an
// error -- the row may have been destroyed under a running instance.
func TestClaimDeclinesForMissingWorker(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)
	missingId := workerId + 1

	// test
	instance, err := workers.ClaimInstance(t.Context(), missingId, time.Minute)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if instance != nil {
		t.Fatalf("ClaimInstance(%d) for a missing worker = %+v, want nil", missingId, instance)
	}
}

// invariant (one live lease): only live rows count toward target_instances,
// so an expired instance frees its place without being deleted.
func TestExpiredInstanceDoesNotCountTowardTarget(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)
	held := claimInstance(t, workers, workerId)

	// test
	instance, err := workers.ClaimInstance(t.Context(), workerId, time.Minute)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if instance != nil {
		t.Fatalf("ClaimInstance(%d) with one live instance at target 1 = %+v, want nil", workerId, instance)
	}

	// test: the held lease expires
	expireInstance(t, workers, held.Id)
	instance, err = workers.ClaimInstance(t.Context(), workerId, time.Minute)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if instance == nil {
		t.Fatalf("ClaimInstance(%d) after the held instance expired = nil, want a row", workerId)
	}
}

// invariant (one live lease): a token the row does not hold is refused by
// every verb, and the row is left as it was -- a stale process cannot touch
// its replacement's row.
func TestWrongTokenIsInstanceLost(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)
	held := claimInstance(t, workers, workerId)
	stale := uuid.New()
	ctx := t.Context()

	// test
	renewErr := workers.RenewInstance(ctx, held.Id, stale, time.Hour)
	successErr := workers.RecordInstanceSuccess(ctx, held.Id, stale)
	_, failureErr := workers.RecordInstanceFailure(ctx, held.Id, stale)
	releaseErr := workers.ReleaseInstance(ctx, held.Id, stale)

	// verify: the claim gave the lease one minute; the refused renew asked for an hour
	if !errors.Is(renewErr, worker.ErrInstanceLost) {
		t.Errorf("RenewInstance with a stale token = %v, want ErrInstanceLost", renewErr)
	}
	if !errors.Is(successErr, worker.ErrInstanceLost) {
		t.Errorf("RecordInstanceSuccess with a stale token = %v, want ErrInstanceLost", successErr)
	}
	if !errors.Is(failureErr, worker.ErrInstanceLost) {
		t.Errorf("RecordInstanceFailure with a stale token = %v, want ErrInstanceLost", failureErr)
	}
	if !errors.Is(releaseErr, worker.ErrInstanceLost) {
		t.Errorf("ReleaseInstance with a stale token = %v, want ErrInstanceLost", releaseErr)
	}
	var attempts int
	var withinMinute bool
	instances := workers.Datastore.Schema + ".worker_instance"
	if err := workers.Datastore.Pool.QueryRow(ctx, "SELECT attempts, expires_at <= now() + interval '1 minute' FROM "+instances+" WHERE id = $1", held.Id).Scan(&attempts, &withinMinute); err != nil {
		t.Fatalf("instance row after stale-token verbs: %v, want it present", err)
	}
	if attempts != 0 || !withinMinute {
		t.Fatalf("instance after stale-token verbs = (attempts %d, lease within a minute %t), want (0, true)", attempts, withinMinute)
	}
}

// behavior: once an instance is released its row is gone, so every later
// verb on it returns ErrInstanceLost.
func TestReleasedInstanceIsInstanceLost(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)
	held := claimInstance(t, workers, workerId)
	token := uuid.UUID(held.Token.Bytes)
	ctx := t.Context()
	if err := workers.ReleaseInstance(ctx, held.Id, token); err != nil {
		t.Fatal(err)
	}

	// test
	renewErr := workers.RenewInstance(ctx, held.Id, token, time.Hour)
	successErr := workers.RecordInstanceSuccess(ctx, held.Id, token)
	_, failureErr := workers.RecordInstanceFailure(ctx, held.Id, token)
	releaseErr := workers.ReleaseInstance(ctx, held.Id, token)

	// verify
	if !errors.Is(renewErr, worker.ErrInstanceLost) {
		t.Errorf("RenewInstance after release = %v, want ErrInstanceLost", renewErr)
	}
	if !errors.Is(successErr, worker.ErrInstanceLost) {
		t.Errorf("RecordInstanceSuccess after release = %v, want ErrInstanceLost", successErr)
	}
	if !errors.Is(failureErr, worker.ErrInstanceLost) {
		t.Errorf("RecordInstanceFailure after release = %v, want ErrInstanceLost", failureErr)
	}
	if !errors.Is(releaseErr, worker.ErrInstanceLost) {
		t.Errorf("ReleaseInstance after release = %v, want ErrInstanceLost", releaseErr)
	}
}

// invariant (one live lease): an expired instance cannot renew, because its
// place may already be held by a replacement; it can still release its row.
func TestExpiredInstanceCannotRenewButCanRelease(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)
	held := claimInstance(t, workers, workerId)
	token := uuid.UUID(held.Token.Bytes)
	expireInstance(t, workers, held.Id)

	// test
	err := workers.RenewInstance(t.Context(), held.Id, token, time.Hour)

	// verify
	if !errors.Is(err, worker.ErrInstanceLost) {
		t.Fatalf("RenewInstance after expiry = %v, want ErrInstanceLost", err)
	}

	// test
	err = workers.ReleaseInstance(t.Context(), held.Id, token)

	// verify
	if err != nil {
		t.Fatalf("ReleaseInstance after expiry = %v, want nil", err)
	}
}

// behavior: a release frees the instance's place immediately, so a
// replacement claims at once instead of waiting out the lease.
func TestReleaseThenClaimAtTargetOneSucceeds(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)
	held := claimInstance(t, workers, workerId)
	token := uuid.UUID(held.Token.Bytes)
	ctx := t.Context()
	if err := workers.ReleaseInstance(ctx, held.Id, token); err != nil {
		t.Fatal(err)
	}

	// test
	instance, err := workers.ClaimInstance(ctx, workerId, time.Minute)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if instance == nil {
		t.Fatalf("ClaimInstance(%d) after release = nil, want a row", workerId)
	}
	if instance.Id == held.Id {
		t.Fatalf("ClaimInstance(%d) after release reused row %d, want a new row", workerId, held.Id)
	}
}

// invariant (crash consistency): a claim commits its worker_instance row
// and its worker_instance_log row together, or neither.
func TestClaimDoesNotCommitWithoutItsLogRow(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", -1)
	rejectInstanceLogInserts(t, workers)
	ctx := t.Context()

	// test
	instance, err := workers.ClaimInstance(ctx, workerId, time.Minute)

	// verify
	if err == nil {
		t.Fatalf("ClaimInstance(%d) with log inserts rejected = %+v, want an error", workerId, instance)
	}
	var live int
	instances := workers.Datastore.Schema + ".worker_instance"
	if err := workers.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+instances+" WHERE worker_id = $1", workerId).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Fatalf("worker_instance rows after the failed claim = %d, want 0", live)
	}
}

// invariant (crash consistency): a renewal commits its new expires_at and
// its worker_instance_log row together, or neither.
func TestRenewDoesNotCommitWithoutItsLogRow(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)
	held := claimInstance(t, workers, workerId)
	token := uuid.UUID(held.Token.Bytes)
	rejectInstanceLogInserts(t, workers)
	ctx := t.Context()

	// test
	err := workers.RenewInstance(ctx, held.Id, token, time.Hour)

	// verify: the claim gave the lease one minute; the failed renew asked for an hour
	if err == nil {
		t.Fatal("RenewInstance with log inserts rejected = nil, want an error")
	}
	var withinMinute bool
	instances := workers.Datastore.Schema + ".worker_instance"
	if err := workers.Datastore.Pool.QueryRow(ctx, "SELECT expires_at <= now() + interval '1 minute' FROM "+instances+" WHERE id = $1", held.Id).Scan(&withinMinute); err != nil {
		t.Fatal(err)
	}
	if !withinMinute {
		t.Fatalf("lease within a minute after the failed renew = %t, want true", withinMinute)
	}
}

// behavior: each recorded failure adds one to attempts and returns the new
// count; a recorded success sets it back to 0.
func TestFailureCountIncrementsAndSuccessResetsIt(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)
	held := claimInstance(t, workers, workerId)
	token := uuid.UUID(held.Token.Bytes)
	ctx := t.Context()

	// test
	first, firstErr := workers.RecordInstanceFailure(ctx, held.Id, token)
	second, secondErr := workers.RecordInstanceFailure(ctx, held.Id, token)
	successErr := workers.RecordInstanceSuccess(ctx, held.Id, token)

	// verify
	if firstErr != nil || secondErr != nil || successErr != nil {
		t.Fatalf("record verbs = %v, %v, %v; want nil, nil, nil", firstErr, secondErr, successErr)
	}
	if first != 1 || second != 2 {
		t.Fatalf("RecordInstanceFailure twice = %d, %d; want 1, 2", first, second)
	}
	var attempts int
	instances := workers.Datastore.Schema + ".worker_instance"
	if err := workers.Datastore.Pool.QueryRow(ctx, "SELECT attempts FROM "+instances+" WHERE id = $1", held.Id).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 {
		t.Fatalf("attempts after RecordInstanceSuccess = %d, want 0", attempts)
	}
}

// behavior: the sweep deletes only rows past expires_at and returns how many
// it deleted; live rows stay.
func TestSweepRemovesOnlyExpiredInstances(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", -1)
	live := claimInstance(t, workers, workerId)
	expireInstance(t, workers, claimInstance(t, workers, workerId).Id)
	expireInstance(t, workers, claimInstance(t, workers, workerId).Id)
	ctx := t.Context()

	// test
	removed, err := workers.SweepExpiredInstances(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("SweepExpiredInstances() = %d, want 2", removed)
	}
	var remaining int64
	instances := workers.Datastore.Schema + ".worker_instance"
	if err := workers.Datastore.Pool.QueryRow(ctx, "SELECT min(id) FROM "+instances+" WHERE worker_id = $1", workerId).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != live.Id {
		t.Fatalf("worker_instance row left after the sweep = %d, want the live row %d", remaining, live.Id)
	}
}
