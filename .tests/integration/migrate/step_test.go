package migrate

import (
	"errors"
	"testing"

	"github.com/agentstax/sqlstreams/pkg/migrate/controller/datastore"
)

// Invariant: the migration lock is a session lock -- it survives the
// transactions the steps commit on its connection, and only ReleaseLock
// frees it.
func TestAcquireLockHoldsAcrossAStepsTransaction(t *testing.T) {
	// setup
	migrations, system := newMigrateDatastore(t)
	ctx := t.Context()
	owner := systemOwner(t, system)
	step := transactionalStep(t, 2, 0, applyNothing)

	// test
	before, err := migrations.IsLocked(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if before {
		t.Error("IsLocked() before AcquireLock = true, want false")
	}

	// test
	conn, err := migrations.AcquireLock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	held, err := migrations.IsLocked(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Error("IsLocked() after AcquireLock = false, want true")
	}

	// test
	if err := migrations.RunStep(ctx, conn, owner, step); err != nil {
		t.Fatal(err)
	}
	afterStep, err := migrations.IsLocked(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if !afterStep {
		t.Error("IsLocked() after a step committed on the lock's connection = false, want true")
	}

	// test
	migrations.ReleaseLock(ctx, conn)
	released, err := migrations.IsLocked(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if released {
		t.Error("IsLocked() after ReleaseLock = true, want false")
	}
}

// Invariant: a transactional step commits its DDL and its success row
// together -- a failing apply leaves neither, and the recorded failure never
// moves the owner's version.
func TestFailingStepLeavesNeitherItsDdlNorASuccessRow(t *testing.T) {
	// setup
	migrations, system := newMigrateDatastore(t)
	ctx := t.Context()
	owner := systemOwner(t, system)
	conn := acquireLock(t, migrations)
	failing := transactionalStep(t, 2, 0, createVersionIndexThenFail)
	succeeding := transactionalStep(t, 2, 0, createVersionIndex)

	// test
	err := migrations.RunStep(ctx, conn, owner, failing)

	// verify
	if !errors.Is(err, errStepFailed) {
		t.Fatalf("RunStep(failing) error = %v, want errStepFailed", err)
	}
	if indexExists(t, migrations, versionIndex) {
		t.Errorf("index %s after the failed step = present, want rolled back", versionIndex)
	}
	if count := countMigrationRows(t, migrations, system.Id, "success"); count != 1 {
		t.Errorf("success rows after the failed step = %d, want the baseline only", count)
	}

	// test
	migrations.TryRecordFailure(ctx, conn, owner, 2, err)

	// verify
	var recordedVersion int64
	var recordedError string
	if err := migrations.Datastore.Pool.QueryRow(ctx, "SELECT version, error FROM "+migrations.Datastore.Schema+".migration_log WHERE status = 'failure'").Scan(&recordedVersion, &recordedError); err != nil {
		t.Fatal(err)
	}
	if recordedVersion != 2 || recordedError != err.Error() {
		t.Errorf("failure row = version %d, error %q, want version 2 and the step's error", recordedVersion, recordedError)
	}
	version, err := datastore.Version(ctx, migrations.Datastore.Pool, owner, migrations.Datastore.Schema)
	if err != nil {
		t.Fatal(err)
	}
	if version != 1 {
		t.Errorf("Version() after the recorded failure = %d, want the baseline 1", version)
	}

	// test
	err = migrations.RunStep(ctx, conn, owner, succeeding)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if !indexExists(t, migrations, versionIndex) {
		t.Errorf("index %s after the succeeding step = missing, want created", versionIndex)
	}
	version, err = datastore.Version(ctx, migrations.Datastore.Pool, owner, migrations.Datastore.Schema)
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Errorf("Version() after the succeeding step = %d, want 2", version)
	}
}
