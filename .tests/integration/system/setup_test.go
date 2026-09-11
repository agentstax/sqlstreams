package system

import (
	"testing"

	"github.com/agentstax/sqlstreams/.tests/integration/postgres"
	"github.com/agentstax/sqlstreams/pkg/system/controller/datastore"
)

// controlPlaneTables are the shared tables Register creates and Delete
// drops.
var controlPlaneTables = []string{
	"system_config",
	"stream_config",
	"stream_config_log",
	"consumer_group_config",
	"worker_config",
	"worker_config_log",
	"worker_instance",
	"worker_instance_log",
	"schedule_config",
	"schedule_cursor",
	"migration_log",
}

// newSystemDatastore is the system datastore over a fresh, unregistered
// schema.
func newSystemDatastore(t testing.TB) *datastore.SystemDatastore {
	t.Helper()
	ds := postgres.Start(t)
	systems, err := datastore.NewSystemDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return systems
}

// tableExists reports whether the schema holds a table of that name.
func tableExists(t testing.TB, systems *datastore.SystemDatastore, name string) bool {
	t.Helper()
	var relation *string
	if err := systems.Datastore.Pool.QueryRow(t.Context(), "SELECT to_regclass($1)::text", systems.Datastore.Schema+"."+name).Scan(&relation); err != nil {
		t.Fatal(err)
	}
	return relation != nil
}

// countRows counts the rows of a control-plane table.
func countRows(t testing.TB, systems *datastore.SystemDatastore, table string) int {
	t.Helper()
	var count int
	if err := systems.Datastore.Pool.QueryRow(t.Context(), "SELECT count(*) FROM "+systems.Datastore.Schema+"."+table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// countBaselines counts the system's version-1 success rows in
// migration_log.
func countBaselines(t testing.TB, systems *datastore.SystemDatastore, systemId int64) int {
	t.Helper()
	var count int
	if err := systems.Datastore.Pool.QueryRow(t.Context(), "SELECT count(*) FROM "+systems.Datastore.Schema+".migration_log WHERE system_id = $1 AND version = 1 AND status = 'success'", systemId).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
