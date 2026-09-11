package system

import (
	"reflect"
	"testing"

	"golang.org/x/sync/errgroup"
)

// Invariant: Register seeds the singleton row and its version-1 baseline
// together, and a second Register returns the same row without a second
// baseline.
func TestRegisterSeedsTheSingletonRowAndBaselineOnce(t *testing.T) {
	// setup
	systems := newSystemDatastore(t)
	ctx := t.Context()

	// test
	first, err := systems.Register(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if first.Id == 0 {
		t.Fatalf("Register() = %+v, want a row with an id", first)
	}
	stored, err := systems.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored, first) {
		t.Errorf("Get() after Register = %+v, want the registered row %+v", stored, first)
	}
	if count := countBaselines(t, systems, first.Id); count != 1 {
		t.Errorf("baseline rows after Register = %d, want 1", count)
	}

	// test
	second, err := systems.Register(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, first) {
		t.Errorf("Register() again = %+v, want the same row %+v", second, first)
	}
	if count := countBaselines(t, systems, first.Id); count != 1 {
		t.Errorf("baseline rows after the second Register = %d, want 1", count)
	}
}

// Invariant: concurrent first registrations leave one system row and one
// baseline -- every registrant resolves the same id.
func TestConcurrentFirstRegistrationsLeaveOneSystem(t *testing.T) {
	// setup
	systems := newSystemDatastore(t)
	ctx := t.Context()
	const registrants = 8
	ids := make(chan int64, registrants)

	// test
	var group errgroup.Group
	for range registrants {
		group.Go(func() error {
			registered, err := systems.Register(ctx)
			if err != nil {
				return err
			}
			ids <- registered.Id
			return nil
		})
	}
	err := group.Wait()

	// verify
	if err != nil {
		t.Fatal(err)
	}
	close(ids)
	stored, err := systems.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil {
		t.Fatal("Get() after the concurrent registrations = nil, want the row")
	}
	for id := range ids {
		if id != stored.Id {
			t.Errorf("Register() returned id %d, want the stored row's id %d", id, stored.Id)
		}
	}
	if count := countRows(t, systems, "system_config"); count != 1 {
		t.Errorf("system_config rows after the concurrent registrations = %d, want 1", count)
	}
	if count := countBaselines(t, systems, stored.Id); count != 1 {
		t.Errorf("baseline rows after the concurrent registrations = %d, want 1", count)
	}
}

// Behavior: Delete drops every control-plane table, Get then reads the
// missing table as no system, and Register afterwards starts fresh.
func TestDeleteDropsTheControlPlaneAndRegisterRecreatesIt(t *testing.T) {
	// setup
	systems := newSystemDatastore(t)
	ctx := t.Context()
	if _, err := systems.Register(ctx); err != nil {
		t.Fatal(err)
	}

	// test
	err := systems.Delete(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range controlPlaneTables {
		if tableExists(t, systems, table) {
			t.Errorf("table %s after Delete = present, want dropped", table)
		}
	}
	deleted, err := systems.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != nil {
		t.Errorf("Get() after Delete = %+v, want nil", deleted)
	}

	// test
	registered, err := systems.Register(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range controlPlaneTables {
		if !tableExists(t, systems, table) {
			t.Errorf("table %s after the second Register = missing, want created", table)
		}
	}
	if count := countBaselines(t, systems, registered.Id); count != 1 {
		t.Errorf("baseline rows after the second Register = %d, want 1", count)
	}
}
