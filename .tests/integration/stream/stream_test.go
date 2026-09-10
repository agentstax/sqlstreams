package stream

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
	"golang.org/x/sync/errgroup"
)

// invariant (crash consistency): a first registration creates the stream's
// first partition, its stream_config_log row, and its migration_log baseline
// in one transaction -- a registration whose baseline cannot be written
// leaves no stream behind.
func TestRegisterCreatesThePartitionTheLogRowAndTheMigrationBaselineTogether(t *testing.T) {
	// setup
	streams, system := newStreamDatastore(t)
	ctx := t.Context()
	logs := streams.Datastore.Schema + ".stream_config_log"
	migrations := streams.Datastore.Schema + ".migration_log"

	// test
	registered, err := streams.Register(ctx, declaredStream(system.Id, "orders", 2), "stream_test")

	// verify
	if err != nil {
		t.Fatal(err)
	}
	partition := streams.Datastore.Schema + "." + stream.MessageLogPartitionTable(registered.Id, 0)
	var created bool
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", partition).Scan(&created); err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Errorf("Register(orders) left partition 0 absent, want %s created", partition)
	}
	var logged int
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE stream_id = $1", registered.Id).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 1 {
		t.Errorf("stream_config_log rows after Register(orders) = %d, want 1", logged)
	}
	var baselines int
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+migrations+" WHERE stream_id = $1 AND version = 1 AND status = 'success'", registered.Id).Scan(&baselines); err != nil {
		t.Fatal(err)
	}
	if baselines != 1 {
		t.Errorf("migration_log baseline rows after Register(orders) = %d, want 1", baselines)
	}

	// test: the baseline insert fails
	rejectMigrationLogInserts(t, streams)
	interrupted, err := streams.Register(ctx, declaredStream(system.Id, "shipments", 2), "stream_test")

	// verify
	if err == nil {
		t.Fatalf("Register(shipments) with migration_log rejecting inserts = %+v, want an error", interrupted)
	}
	found, err := streams.Get(ctx, "shipments")
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Errorf("Get(shipments) after the interrupted registration = %+v, want nil", found)
	}
	var tables int
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname = $1 AND tablename LIKE 'message_log_%'", streams.Datastore.Schema).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 2 {
		t.Errorf("message_log tables after the interrupted registration = %d, want orders' parent and partition only, 2", tables)
	}
}

// behavior (newest declaration wins): registering again with the same config
// returns the stored row and writes nothing; a changed mutable config
// replaces the row and appends one stream_config_log row; a different
// partition size is ErrStreamConfigMismatch and writes nothing.
func TestRegisterAgainReplacesMutableConfigButNeverThePartitionSize(t *testing.T) {
	// setup
	streams, system := newStreamDatastore(t)
	ctx := t.Context()
	logs := streams.Datastore.Schema + ".stream_config_log"
	created, err := streams.Register(ctx, declaredStream(system.Id, "orders", 2), "stream_test")
	if err != nil {
		t.Fatal(err)
	}
	changed := declaredStream(system.Id, "orders", 2)
	changed.RetentionTTLNs = int64(time.Hour)
	changed.DeliveryLogMode = string(stream.DeliveryLogModeAll)

	// test
	same, sameErr := streams.Register(ctx, declaredStream(system.Id, "orders", 2), "stream_test")
	replaced, replacedErr := streams.Register(ctx, changed, "stream_test")
	mismatched, mismatchErr := streams.Register(ctx, declaredStream(system.Id, "orders", 4), "stream_test")

	// verify
	if sameErr != nil {
		t.Fatal(sameErr)
	}
	if !reflect.DeepEqual(same, created) {
		t.Errorf("Register(orders, same config) = %+v, want the stored row %+v", same, created)
	}
	if replacedErr != nil {
		t.Fatal(replacedErr)
	}
	if replaced.Id != created.Id || replaced.RetentionTTLNs != int64(time.Hour) || replaced.DeliveryLogMode != string(stream.DeliveryLogModeAll) || replaced.PartitionSize != 2 {
		t.Errorf("Register(orders, changed config) = %+v, want id %d with retention 1h, delivery log mode all, partition size 2", replaced, created.Id)
	}
	if !errors.Is(mismatchErr, stream.ErrStreamConfigMismatch) {
		t.Errorf("Register(orders, partition size 4) = %+v, %v; want ErrStreamConfigMismatch", mismatched, mismatchErr)
	}
	stored, err := streams.Get(ctx, "orders")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored, replaced) {
		t.Errorf("Get(orders) after the three registrations = %+v, want the replaced row %+v", stored, replaced)
	}
	var logged int
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE stream_id = $1", created.Id).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 2 {
		t.Errorf("stream_config_log rows after the three registrations = %d, want the creation and the replacement, 2", logged)
	}
}

// invariant (concurrency): concurrent first registrations of one name all
// return the same row and leave one stream_config row, one log row, and one
// table set behind -- the per-name advisory lock serializes the creation.
func TestConcurrentFirstRegistrationsLeaveOneStream(t *testing.T) {
	// setup
	streams, system := newStreamDatastore(t)
	ctx := t.Context()
	logs := streams.Datastore.Schema + ".stream_config_log"
	const registrants = 8
	ids := make(chan int64, registrants)

	// test
	var group errgroup.Group
	for range registrants {
		group.Go(func() error {
			registered, err := streams.Register(ctx, declaredStream(system.Id, "orders", 2), "stream_test")
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
	stored, err := streams.Get(ctx, "orders")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil {
		t.Fatal("Get(orders) after the concurrent registrations = nil, want the row")
	}
	for id := range ids {
		if id != stored.Id {
			t.Errorf("Register(orders) returned id %d, want the stored row's id %d", id, stored.Id)
		}
	}
	listed, err := streams.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Errorf("List() after the concurrent registrations = %+v, want one row", listed)
	}
	var logged int
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE stream_id = $1", stored.Id).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 1 {
		t.Errorf("stream_config_log rows after the concurrent registrations = %d, want 1", logged)
	}
}

// behavior: a rename keeps the stream's id and tables, moves its name, and
// appends one stream_config_log row under the new name; a taken target is
// ErrStreamNameTaken with nothing changed; a missing source is (nil, nil).
func TestRenameMovesTheNameAndKeepsTheId(t *testing.T) {
	// setup
	streams, system := newStreamDatastore(t)
	ctx := t.Context()
	logs := streams.Datastore.Schema + ".stream_config_log"
	orders, err := streams.Register(ctx, declaredStream(system.Id, "orders", 2), "stream_test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := streams.Register(ctx, declaredStream(system.Id, "shipments", 2), "stream_test"); err != nil {
		t.Fatal(err)
	}

	// test
	renamed, renameErr := streams.Rename(ctx, "orders", "invoices", "stream_test")
	taken, takenErr := streams.Rename(ctx, "invoices", "shipments", "stream_test")
	missing, missingErr := streams.Rename(ctx, "returns", "credits", "stream_test")

	// verify
	if renameErr != nil {
		t.Fatal(renameErr)
	}
	if renamed.Id != orders.Id || renamed.Name != "invoices" {
		t.Errorf("Rename(orders, invoices) = %+v, want id %d under invoices", renamed, orders.Id)
	}
	if !errors.Is(takenErr, stream.ErrStreamNameTaken) {
		t.Errorf("Rename(invoices, shipments) = %+v, %v; want ErrStreamNameTaken", taken, takenErr)
	}
	if missing != nil || missingErr != nil {
		t.Errorf("Rename(returns, credits) = %+v, %v; want nil, nil", missing, missingErr)
	}
	old, err := streams.Get(ctx, "orders")
	if err != nil {
		t.Fatal(err)
	}
	if old != nil {
		t.Errorf("Get(orders) after the rename = %+v, want nil", old)
	}
	stored, err := streams.GetById(ctx, orders.Id)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Name != "invoices" {
		t.Errorf("GetById(%d) after the renames = %+v, want the row under invoices", orders.Id, stored)
	}
	var newest string
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT name FROM "+logs+" WHERE stream_id = $1 ORDER BY id DESC LIMIT 1", orders.Id).Scan(&newest); err != nil {
		t.Fatal(err)
	}
	if newest != "invoices" {
		t.Errorf("newest stream_config_log name for stream %d = %q, want invoices", orders.Id, newest)
	}
	var logged int
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE stream_id = $1", orders.Id).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 2 {
		t.Errorf("stream_config_log rows for stream %d = %d, want the creation and the rename, 2", orders.Id, logged)
	}
}

// behavior: a delete drops every one of the stream's tables and partitions,
// draining more partitions than one drop batch holds, removes its
// stream_config row with the log and consumer group rows under it, and
// leaves a sibling stream alone.
func TestDeleteDropsEveryTableAcrossPartitionBatches(t *testing.T) {
	// setup
	streams, system := newStreamDatastore(t)
	ctx := t.Context()
	logs := streams.Datastore.Schema + ".stream_config_log"
	groups := streams.Datastore.Schema + ".consumer_group_config"
	orders, err := streams.Register(ctx, declaredStream(system.Id, "orders", 2), "stream_test")
	if err != nil {
		t.Fatal(err)
	}
	shipments, err := streams.Register(ctx, declaredStream(system.Id, "shipments", 2), "stream_test")
	if err != nil {
		t.Fatal(err)
	}
	registerConsumerGroup(t, streams, orders.Id, "processor")
	produceMessages(t, streams.Datastore, orders.Id, orders.PartitionSize, 202)
	produceMessages(t, streams.Datastore, shipments.Id, shipments.PartitionSize, 2)

	// test
	err = streams.Delete(ctx, orders.Id, orders.Name)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range streamTables(streams, orders.Id) {
		var present bool
		if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&present); err != nil {
			t.Fatal(err)
		}
		if present {
			t.Errorf("Delete(orders) left %s, want it dropped", table)
		}
	}
	var partitions int
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname = $1 AND tablename LIKE $2", streams.Datastore.Schema, stream.MessageLogTable(orders.Id)+"_%").Scan(&partitions); err != nil {
		t.Fatal(err)
	}
	if partitions != 0 {
		t.Errorf("partitions of %s after Delete(orders) = %d, want 0", stream.MessageLogTable(orders.Id), partitions)
	}
	deleted, err := streams.GetById(ctx, orders.Id)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != nil {
		t.Errorf("GetById(%d) after Delete(orders) = %+v, want nil", orders.Id, deleted)
	}
	var logged int
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE stream_id = $1", orders.Id).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 0 {
		t.Errorf("stream_config_log rows for stream %d after Delete(orders) = %d, want 0", orders.Id, logged)
	}
	var grouped int
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+groups+" WHERE stream_id = $1", orders.Id).Scan(&grouped); err != nil {
		t.Fatal(err)
	}
	if grouped != 0 {
		t.Errorf("consumer_group_config rows for stream %d after Delete(orders) = %d, want 0", orders.Id, grouped)
	}
	for _, table := range streamTables(streams, shipments.Id) {
		var present bool
		if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&present); err != nil {
			t.Fatal(err)
		}
		if !present {
			t.Errorf("Delete(orders) dropped the sibling's %s, want it kept", table)
		}
	}
	var siblingMessages int
	if err := streams.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+streams.Datastore.Schema+"."+stream.MessageLogTable(shipments.Id)).Scan(&siblingMessages); err != nil {
		t.Fatal(err)
	}
	if siblingMessages != 2 {
		t.Errorf("messages in the sibling's log after Delete(orders) = %d, want 2", siblingMessages)
	}
}

// behavior: on a database whose system tables do not exist yet, List and
// Get report no streams instead of a missing-table error -- the CLI's
// first contact with an unregistered database.
func TestListAndGetOnAnUnregisteredDatabaseReturnNothing(t *testing.T) {
	// setup
	streams := newUnregisteredStreamDatastore(t)
	ctx := t.Context()

	// test
	listed, listErr := streams.List(ctx)
	found, getErr := streams.Get(ctx, "orders")
	foundById, getByIdErr := streams.GetById(ctx, 1)

	// verify
	if listed != nil || listErr != nil {
		t.Errorf("List() on an unregistered database = %+v, %v; want nil, nil", listed, listErr)
	}
	if found != nil || getErr != nil {
		t.Errorf("Get(orders) on an unregistered database = %+v, %v; want nil, nil", found, getErr)
	}
	if foundById != nil || getByIdErr != nil {
		t.Errorf("GetById(1) on an unregistered database = %+v, %v; want nil, nil", foundById, getByIdErr)
	}
}
