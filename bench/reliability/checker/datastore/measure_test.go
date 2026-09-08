package datastore

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run only against a disposable database: CreateTables replaces lab's records.
func TestMeasurementsWithOverlappingTopicIds(t *testing.T) {
	connection := os.Getenv("RELIABILITY_TEST_DSN")
	if connection == "" {
		t.Skip("RELIABILITY_TEST_DSN must name a disposable database")
	}
	ctx := context.Background()
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
		(at, kind, topic, producer, sequence, key, scheduled_at, message_id, duplicate, code, error)
		VALUES
		('2026-09-07 00:00:00Z', 'committed', 'orders', 'p', 1, 'p-1', '2026-09-07 00:00:00Z', 7, false, '', ''),
		('2026-09-07 00:00:00Z', 'committed', 'invoices', 'p', 1, 'p-1', '2026-09-07 00:00:00Z', 7, false, '', '');
		INSERT INTO lab.handler_record (at, consumer, topic, "group", message_id, key, attempt, outcome)
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
		t.Fatalf("topic/group latencies = %+v; want four deliveries, median 2.5s, max 4s", measured)
	}
}

func TestCompletionWithoutSuccessAuditRows(t *testing.T) {
	connection := os.Getenv("RELIABILITY_TEST_DSN")
	if connection == "" {
		t.Skip("RELIABILITY_TEST_DSN must name a disposable database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ds, err := NewCheckerDatastore(pool)
	if err != nil {
		t.Fatal(err)
	}
	target := Target{Topic: "fixture", Group: "processor", TopicId: 98765, GroupId: 1}
	_, err = pool.Exec(ctx, `
		CREATE SCHEMA IF NOT EXISTS vulkan;
		DROP TABLE IF EXISTS vulkan.message_log_98765, vulkan.consumer_group_cursor_98765, vulkan.exception_queue_98765;
		CREATE TABLE vulkan.message_log_98765 (id bigint PRIMARY KEY);
		CREATE TABLE vulkan.consumer_group_cursor_98765 (consumer_group_id bigint, committed bigint);
		CREATE TABLE vulkan.exception_queue_98765 (consumer_group_id bigint, message_id bigint, status text);
		INSERT INTO vulkan.message_log_98765 VALUES (7), (8), (9);
		INSERT INTO vulkan.consumer_group_cursor_98765 VALUES (1, 7);
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
		UPDATE vulkan.consumer_group_cursor_98765 SET committed = 9;
		INSERT INTO vulkan.exception_queue_98765 VALUES (1, 8, 'dead');
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
	if _, err = pool.Exec(ctx, "INSERT INTO vulkan.exception_queue_98765 VALUES (1, 9, 'inflight')"); err != nil {
		t.Fatal(err)
	}
	measured, err = ds.CountUnbucketed(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if measured.Count != 1 {
		t.Fatalf("unfinished exception count = %d, want 1", measured.Count)
	}
	if _, err = pool.Exec(ctx, "DELETE FROM vulkan.consumer_group_cursor_98765"); err != nil {
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
