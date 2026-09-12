package stream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	vacuumdatastore "github.com/allegedlyreliable/sqlstreams/pkg/stream/vacuum/controller/datastore"
)

func TestVacuumRefreshesKeyStatisticsWithoutDeletingRetainedKeys(t *testing.T) {
	// setup
	janitor, orders := newRetentionJanitor(t)
	keys := janitor.Datastore.Schema + "." + stream.IdempotencyKeyTable(orders.Id)
	seedRetentionKeys(t, janitor, keys)
	if err := janitor.SweepExpiredIdempotencyKeys(t.Context(), orders.Id, time.Hour, 2); err != nil {
		t.Fatal(err)
	}
	vacuum, err := vacuumdatastore.NewVacuumDatastore(janitor.Datastore, janitor.Logger)
	if err != nil {
		t.Fatal(err)
	}

	// test
	err = vacuum.VacuumIdempotencyKeys(t.Context(), orders.Id)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := janitor.Datastore.Pool.QueryRow(t.Context(), "SELECT count(*) FROM "+keys).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("VacuumIdempotencyKeys(%d) retained %d keys, want 1", orders.Id, count)
	}
	var estimate float64
	if err := janitor.Datastore.Pool.QueryRow(t.Context(), "SELECT reltuples FROM pg_class WHERE oid=to_regclass($1)", keys).Scan(&estimate); err != nil {
		t.Fatal(err)
	}
	if estimate != 1 {
		t.Errorf("VacuumIdempotencyKeys(%d) estimated %v keys, want 1", orders.Id, estimate)
	}
}

func TestVacuumDeadlineCancelsWaitingForTableLock(t *testing.T) {
	// setup
	janitor, orders := newRetentionJanitor(t)
	vacuum, err := vacuumdatastore.NewVacuumDatastore(janitor.Datastore, janitor.Logger)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := janitor.Datastore.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	if _, err := tx.Exec(t.Context(), "LOCK TABLE "+janitor.Datastore.Schema+"."+stream.IdempotencyKeyTable(orders.Id)+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	// test
	err = vacuum.VacuumIdempotencyKeys(ctx, orders.Id)

	// verify
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("VacuumIdempotencyKeys(%d) = %v, want deadline exceeded", orders.Id, err)
	}
}
