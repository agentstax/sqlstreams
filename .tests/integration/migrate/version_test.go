package migrate

import (
	"errors"
	"reflect"
	"testing"

	"github.com/allegedlyreliable/sqlstreams/pkg/migrate"
	"github.com/allegedlyreliable/sqlstreams/pkg/migrate/controller/datastore"
)

// Invariant: an owner's version is its latest-by-id success row, never the
// highest, and its floor is the strictest MinCompatibleVersion at or below
// that row -- a step rolled back below it no longer binds.
func TestSchemaStateReadsTheLatestSuccessAndTheFloorBelowIt(t *testing.T) {
	// setup
	migrations, system := newMigrateDatastore(t)
	ctx := t.Context()
	owner := systemOwner(t, system)
	conn := acquireLock(t, migrations)
	upToTwo := transactionalStep(t, 2, 0, applyNothing)
	upToThree := transactionalStep(t, 3, 3, applyNothing)
	downToTwo := transactionalStep(t, 2, 0, applyNothing)
	orders := registerStream(t, migrations, system.Id, "orders")

	// test
	if err := migrations.RunStep(ctx, conn, owner, upToTwo); err != nil {
		t.Fatal(err)
	}
	if err := migrations.RunStep(ctx, conn, owner, upToThree); err != nil {
		t.Fatal(err)
	}
	atThree, err := migrations.SystemSchemaState(ctx, system.Id)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if want := (datastore.SchemaStateRow{Version: 3, MinCompatibleVersion: 3}); !reflect.DeepEqual(*atThree, want) {
		t.Errorf("SystemSchemaState() after up to 3 = %+v, want %+v", *atThree, want)
	}

	// test
	if err := migrations.RunStep(ctx, conn, owner, downToTwo); err != nil {
		t.Fatal(err)
	}
	atTwo, err := migrations.SystemSchemaState(ctx, system.Id)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if want := (datastore.SchemaStateRow{Version: 2, MinCompatibleVersion: 0}); !reflect.DeepEqual(*atTwo, want) {
		t.Errorf("SystemSchemaState() after down to 2 = %+v, want %+v", *atTwo, want)
	}

	// test
	if err := migrations.RunStep(ctx, conn, streamOwner(t, orders), upToThree); err != nil {
		t.Fatal(err)
	}
	streamState, err := migrations.StreamSchemaState(ctx, orders.Id)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if want := (datastore.SchemaStateRow{Version: 3, MinCompatibleVersion: 3}); !reflect.DeepEqual(*streamState, want) {
		t.Errorf("StreamSchemaState(orders) after up to 3 = %+v, want %+v", *streamState, want)
	}
	systemState, err := migrations.SystemSchemaState(ctx, system.Id)
	if err != nil {
		t.Fatal(err)
	}
	if want := (datastore.SchemaStateRow{Version: 2, MinCompatibleVersion: 0}); !reflect.DeepEqual(*systemState, want) {
		t.Errorf("SystemSchemaState() after the stream's step = %+v, want the system's own %+v", *systemState, want)
	}
}

// Behavior: a database with no tables and a stream with no baseline both
// read as not registered.
func TestUnregisteredOwnersReadAsNotRegistered(t *testing.T) {
	// setup
	unregistered := newUnregisteredMigrateDatastore(t)
	migrations, _ := newMigrateDatastore(t)
	ctx := t.Context()

	// test
	_, ownerErr := unregistered.SystemOwner(ctx)
	_, stateErr := migrations.StreamSchemaState(ctx, 404)

	// verify
	if !errors.Is(ownerErr, migrate.ErrNotRegistered) {
		t.Errorf("SystemOwner() on a database with no tables error = %v, want ErrNotRegistered", ownerErr)
	}
	if !errors.Is(stateErr, migrate.ErrNotRegistered) {
		t.Errorf("StreamSchemaState(404) error = %v, want ErrNotRegistered", stateErr)
	}
}
