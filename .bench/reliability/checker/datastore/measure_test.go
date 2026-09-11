package datastore

import (
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run only against a disposable database: CreateTables replaces lab's records.
func TestMeasurementsWithOverlappingStreamIds(t *testing.T) {
	connection := databaseURL(t)
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ds, err := NewCheckerDatastore(pool)
	if err != nil {
		t.Fatal(err)
	}
	if err := ds.CreateTables(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO lab.produce_record
		(at, kind, stream, producer, sequence, key, scheduled_at, message_id, duplicate, code, error)
		VALUES
		('2026-09-07 00:00:00Z', 'committed', 'orders', 'p', 1, 'p-1', '2026-09-07 00:00:00Z', 7, false, '', ''),
		('2026-09-07 00:00:00Z', 'committed', 'invoices', 'p', 1, 'p-1', '2026-09-07 00:00:00Z', 7, false, '', '');
		INSERT INTO lab.handler_record (at, consumer, stream, "group", message_id, key, attempt, outcome)
		VALUES
		('2026-09-07 00:00:01Z', 'c', 'orders', 'fast', 7, 'p-1', 1, 'success'),
		('2026-09-07 00:00:02Z', 'c', 'orders', 'slow', 7, 'p-1', 1, 'success'),
		('2026-09-07 00:00:03Z', 'c', 'invoices', 'fast', 7, 'p-1', 1, 'success'),
		('2026-09-07 00:00:04Z', 'c', 'invoices', 'slow', 7, 'p-1', 1, 'success'),
		('2026-09-07 00:00:09Z', 'c', 'orders', 'fast', 7, 'p-1', 2, 'success');
	`)
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	measured, err := ds.ReadEndToEndLatency(ctx, from, from.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if measured.Count != 4 || measured.P50 != 2500*time.Millisecond || measured.Max != 4*time.Second {
		t.Fatalf("stream/group latencies = %+v; want four deliveries, median 2.5s, max 4s", measured)
	}
}

func TestCompletionWithoutSuccessAuditRows(t *testing.T) {
	connection := databaseURL(t)
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ds, err := NewCheckerDatastore(pool)
	if err != nil {
		t.Fatal(err)
	}
	target := Target{Stream: "fixture", Group: "processor", StreamId: 98765, GroupId: 1}
	_, err = pool.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS sqlstreams;
		DROP TABLE IF EXISTS sqlstreams.message_log_98765, sqlstreams.consumer_group_cursor_98765, sqlstreams.exception_queue_98765;
		CREATE TABLE sqlstreams.message_log_98765 (id bigint PRIMARY KEY);
		CREATE TABLE sqlstreams.consumer_group_cursor_98765 (consumer_group_id bigint, committed bigint);
		CREATE TABLE sqlstreams.exception_queue_98765 (consumer_group_id bigint, message_id bigint, status text);
		INSERT INTO sqlstreams.message_log_98765 VALUES (7), (8), (9);
		INSERT INTO sqlstreams.consumer_group_cursor_98765 VALUES (1, 7);
	`)
	if err != nil {
		t.Fatal(err)
	}
	measured, err := ds.CountUnbucketed(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if measured.Count != 2 {
		t.Fatalf("unfinished count = %d, want 2", measured.Count)
	}
	_, err = pool.Exec(ctx, `
		UPDATE sqlstreams.consumer_group_cursor_98765 SET committed = 9;
		INSERT INTO sqlstreams.exception_queue_98765 VALUES (1, 8, 'dead');
	`)
	if err != nil {
		t.Fatal(err)
	}
	measured, err = ds.CountUnbucketed(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if measured.Count != 0 {
		t.Fatalf("finished/dead count = %d, want 0", measured.Count)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO sqlstreams.exception_queue_98765 VALUES (1, 9, 'inflight')"); err != nil {
		t.Fatal(err)
	}
	measured, err = ds.CountUnbucketed(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if measured.Count != 1 {
		t.Fatalf("unfinished exception count = %d, want 1", measured.Count)
	}
	if _, err = pool.Exec(ctx, "DELETE FROM sqlstreams.consumer_group_cursor_98765"); err != nil {
		t.Fatal(err)
	}
	measured, err = ds.CountUnbucketed(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if measured.Count != 2 {
		t.Fatalf("missing cursor count = %d, want 2", measured.Count)
	}
}

// databaseURL is the disposable server SQLSTREAMS_TEST_DATABASE_URL names;
// the test skips when it is unset.
func databaseURL(t testing.TB) string {
	t.Helper()
	url := os.Getenv("SQLSTREAMS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SQLSTREAMS_TEST_DATABASE_URL is unset -- set it to a disposable PostgreSQL database URL to run database tests")
	}
	return url
}
