package migrate

import (
	"context"
	"errors"
	"testing"

	"github.com/allegedlyreliable/sqlstreams/.tests/integration/postgres"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/migrate/controller/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	streamcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/system"
	systemcontroller "github.com/allegedlyreliable/sqlstreams/pkg/system/controller"
	"github.com/jackc/pgx/v5/pgxpool"
)

// versionIndex is the index the DDL steps create on migration_log.
const versionIndex = "migration_log_version"

// errStepFailed is what a failing step's apply returns.
var errStepFailed = errors.New("step failed")

// newMigrateDatastore registers a system and returns the migrate datastore
// with it.
func newMigrateDatastore(t testing.TB) (*datastore.MigrateDatastore, *system.System) {
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
	migrations, err := datastore.NewMigrateDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return migrations, registered
}

// newUnregisteredMigrateDatastore is the migrate datastore over a schema
// with no system and no tables.
func newUnregisteredMigrateDatastore(t testing.TB) *datastore.MigrateDatastore {
	t.Helper()
	ds := postgres.Start(t)
	migrations, err := datastore.NewMigrateDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return migrations
}

// registerStream registers a stream on the system through the stream
// controller.
func registerStream(t testing.TB, migrations *datastore.MigrateDatastore, systemId int64, name string) *stream.Stream {
	t.Helper()
	streams, err := streamcontroller.NewStreamController(migrations.Datastore, migrations.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := streams.Register(t.Context(), systemId, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	return registered
}

// systemOwner is the system's owner.
func systemOwner(t testing.TB, registered *system.System) *common.Owner {
	t.Helper()
	owner, err := common.NewSystemOwner(registered.Id)
	if err != nil {
		t.Fatal(err)
	}
	return owner
}

// streamOwner is the stream's owner.
func streamOwner(t testing.TB, registered *stream.Stream) *common.Owner {
	t.Helper()
	owner, err := common.NewStreamOwner(registered.SystemId, registered.Id, registered.Name)
	if err != nil {
		t.Fatal(err)
	}
	return owner
}

// acquireLock takes the migration session lock, released at cleanup.
func acquireLock(t testing.TB, migrations *datastore.MigrateDatastore) *pgxpool.Conn {
	t.Helper()
	conn, err := migrations.AcquireLock(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { migrations.ReleaseLock(context.Background(), conn) })
	return conn
}

// transactionalStep is a step to version with the floor and apply, run in
// its own transaction with no validate.
func transactionalStep(t testing.TB, version int64, minCompatibleVersion int64, apply func(context.Context, iDatastore.Querier, string, int64) error) *datastore.Step {
	t.Helper()
	step, err := datastore.NewStep(version, minCompatibleVersion, nil, apply, false)
	if err != nil {
		t.Fatal(err)
	}
	return step
}

// indexExists reports whether the schema holds an index of that name.
func indexExists(t testing.TB, migrations *datastore.MigrateDatastore, name string) bool {
	t.Helper()
	var relation *string
	if err := migrations.Datastore.Pool.QueryRow(t.Context(), "SELECT to_regclass($1)::text", migrations.Datastore.Schema+"."+name).Scan(&relation); err != nil {
		t.Fatal(err)
	}
	return relation != nil
}

// countMigrationRows counts the system's migration_log rows with the status.
func countMigrationRows(t testing.TB, migrations *datastore.MigrateDatastore, systemId int64, status string) int {
	t.Helper()
	var count int
	if err := migrations.Datastore.Pool.QueryRow(t.Context(), "SELECT count(*) FROM "+migrations.Datastore.Schema+".migration_log WHERE system_id = $1 AND status = $2", systemId, status).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// applyNothing is a step apply that only records its version.
func applyNothing(ctx context.Context, q iDatastore.Querier, schema string, streamId int64) error {
	return nil
}

// createVersionIndex is a step apply that creates versionIndex.
func createVersionIndex(ctx context.Context, q iDatastore.Querier, schema string, streamId int64) error {
	_, err := q.Exec(ctx, "CREATE INDEX "+versionIndex+" ON "+schema+".migration_log (version)")
	return err
}

// createVersionIndexThenFail is a step apply that creates versionIndex and
// then fails.
func createVersionIndexThenFail(ctx context.Context, q iDatastore.Querier, schema string, streamId int64) error {
	if err := createVersionIndex(ctx, q, schema, streamId); err != nil {
		return err
	}
	return errStepFailed
}
