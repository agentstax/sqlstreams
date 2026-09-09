package datastore

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEmptyClaimPersistsPendingObservation(t *testing.T) {
	ds := newClaimTestDatastore(t)
	ctx := t.Context()
	pool := ds.Datastore.Pool
	messages := ds.Datastore.Schema + "." + stream.MessageLogTable(1)
	if _, err := pool.Exec(ctx, "INSERT INTO "+messages+" DEFAULT VALUES; INSERT INTO "+messages+" DEFAULT VALUES"); err != nil {
		t.Fatal(err)
	}

	older := holdClaimTestTransaction(t, pool)
	// Advance the last completed transaction beyond older, so both the old
	// snapshot fence and the replacement fence must wait for it.
	if _, err := pool.Exec(ctx, "SELECT pg_current_xact_id()"); err != nil {
		t.Fatal(err)
	}
	claimed, err := ds.ClaimMessagesWithCursor(ctx, 1, 1, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)
	if err != nil {
		t.Fatal(err)
	}
	if claimed != nil {
		t.Fatalf("claimed while an older transaction is open: %+v", claimed)
	}
	var head int64
	var recorded bool
	if err := pool.QueryRow(ctx, "SELECT pending_head, pending_xmax IS NOT NULL FROM "+ds.Datastore.Schema+"."+stream.ConsumerGroupCursorTable(1)).Scan(&head, &recorded); err != nil {
		t.Fatal(err)
	}
	if head != 2 || !recorded {
		t.Fatalf("empty claim discarded observation: head=%d, recorded=%t; want 2, true", head, recorded)
	}
	if err := older.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// A new open transaction blocks the fresh observation, but must not block
	// the saved observation whose older transactions have all finished.
	holdClaimTestTransaction(t, pool)
	if _, err := pool.Exec(ctx, "SELECT pg_current_xact_id()"); err != nil {
		t.Fatal(err)
	}
	claimed, err = ds.ClaimMessagesWithCursor(ctx, 1, 1, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil || len(claimed.Messages) != 2 || claimed.Lease.Low != 0 || claimed.Lease.High != 2 {
		t.Fatalf("saved observation did not permit both messages: %+v", claimed)
	}
}

func TestClaimWaitsForProducerBeyondSnapshotXmax(t *testing.T) {
	ds := newClaimTestDatastore(t)
	ctx := t.Context()
	pool := ds.Datastore.Pool
	messages := ds.Datastore.Schema + "." + stream.MessageLogTable(1)
	older := holdClaimTestTransaction(t, pool)
	newer := holdClaimTestTransaction(t, pool)
	// Both producers own transaction ids before allocating message ids, as
	// SQLStreams's idempotency write requires. Their allocation orders differ.
	if _, err := newer.Exec(ctx, "INSERT INTO "+messages+" DEFAULT VALUES"); err != nil {
		t.Fatal(err)
	}
	if _, err := older.Exec(ctx, "INSERT INTO "+messages+" DEFAULT VALUES"); err != nil {
		t.Fatal(err)
	}
	if err := older.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var outside bool
	if err := newer.QueryRow(ctx, "SELECT pg_current_xact_id() >= pg_snapshot_xmax(pg_current_snapshot())").Scan(&outside); err != nil {
		t.Fatal(err)
	}
	if !outside {
		t.Fatal("fixture needs an isolated database: another completed transaction advanced snapshot xmax")
	}

	claimed, err := ds.ClaimMessagesWithCursor(ctx, 1, 1, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)
	if err != nil {
		t.Fatal(err)
	}
	if claimed != nil {
		t.Fatalf("claimed past uncommitted message 1: lease=(%d,%d], visible messages=%+v", claimed.Lease.Low, claimed.Lease.High, claimed.Messages)
	}
	if err := newer.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	claimed, err = ds.ClaimMessagesWithCursor(ctx, 1, 1, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil || len(claimed.Messages) != 2 || claimed.Messages[0].Id != 1 || claimed.Messages[1].Id != 2 {
		t.Fatalf("both committed messages must be delivered in order: %+v", claimed)
	}
}

func TestCaughtUpClaimsDoNotAllocateTransactionIds(t *testing.T) {
	ds := newClaimTestDatastore(t)
	ctx := t.Context()
	var before, after int64
	if err := ds.Datastore.Pool.QueryRow(ctx, "SELECT pg_current_xact_id()::text::bigint").Scan(&before); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		claimed, err := ds.ClaimMessagesWithCursor(ctx, 1, 1, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)
		if err != nil || claimed != nil {
			t.Fatalf("caught-up claim=%+v, error=%v", claimed, err)
		}
	}
	if err := ds.Datastore.Pool.QueryRow(ctx, "SELECT pg_current_xact_id()::text::bigint").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before+1 {
		t.Fatalf("caught-up polls allocated transaction ids: before=%d, after=%d", before, after)
	}
}

func newClaimTestDatastore(t *testing.T) *MessageConsumerGroupDatastore {
	t.Helper()
	connection := os.Getenv("SQLSTREAMS_TEST_DSN")
	if connection == "" {
		t.Skip("SQLSTREAMS_TEST_DSN must name an isolated disposable PostgreSQL database")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("claim_test_%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		CREATE SCHEMA %[1]s;
		CREATE TABLE %[1]s.%[2]s (
			id bigserial PRIMARY KEY, payload jsonb NOT NULL DEFAULT '{}',
			created_at timestamptz NOT NULL DEFAULT now(), schema_version integer NOT NULL DEFAULT 1,
			routing_key text, message_key text, compaction_rank bigint, options jsonb
		);
		CREATE TABLE %[1]s.%[3]s (
			consumer_group_id bigint PRIMARY KEY, claimed bigint NOT NULL DEFAULT 0,
			committed bigint NOT NULL DEFAULT 0, settled_head bigint NOT NULL DEFAULT 0,
			pending_head bigint NOT NULL DEFAULT 0, pending_xmax xid8
		);
		INSERT INTO %[1]s.%[3]s (consumer_group_id) VALUES (1);
		CREATE TABLE %[1]s.%[4]s (
			token uuid PRIMARY KEY DEFAULT gen_random_uuid(), consumer_group_id bigint,
			low bigint, high bigint, expires_at timestamptz, reclaims integer NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.%[5]s (consumer_group_id bigint, pattern_regex text);
		CREATE TABLE %[1]s.%[6]s (compaction_key text, message_id bigint);
	`, schema, stream.MessageLogTable(1), stream.ConsumerGroupCursorTable(1), stream.ClaimLeaseTable(1), stream.BindingConfigTable(1), stream.CompactionHeadTable(1)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	ds, err := datastore.NewPostgresDatastore(ctx, pool, &datastore.PostgresDatastoreConfig{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := NewMessageConsumerGroupDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return consumer
}

func holdClaimTestTransaction(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(t.Context(), "SELECT pg_current_xact_id()"); err != nil {
		t.Fatal(err)
	}
	return tx
}
