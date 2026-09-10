package main

// Compaction-head lock lifecycle e2e test: the lockable row serializes the first
// read-modify-write, an ordinary compacted produce fills that same row, and
// the stream janitor removes only idle rows without heads. The final two races
// force each side to win first and prove the janitor never waits on an active
// locker while a lock following a delete safely recreates the row.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	janitorcontroller "github.com/agentstax/sqlstreams/pkg/stream/janitor/controller"
	"github.com/jackc/pgx/v5"
)

const (
	emptyHeadTTL = 100 * time.Millisecond
	batchSize    = 1
)

type Counter struct {
	Value int `json:"value"`
}

func (Counter) SchemaVersion() int { return 1 }

type headRow struct {
	MessageId *int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type testKey struct {
	name   string
	handle *sqlstreams.KeyHandle[Counter]
}

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

type testFailure struct {
	message string
}

func (f testFailure) Error() string { return f.message }

func run() (err error) {
	defer func() {
		switch recovered := recover().(type) {
		case nil:
		case testFailure:
			err = recovered
		default:
			panic(recovered)
		}
	}()

	ctx := context.Background()
	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	must(err)

	streamName := fmt.Sprintf("compactionheadlock.%d", time.Now().UnixNano())
	counters := client.Stream[Counter](streamName)
	registered, err := counters.Register(ctx, &sqlstreams.StreamConfig{
		PartitionSize:          1000,
		EmptyCompactionHeadTTL: emptyHeadTTL,
	})
	must(err)
	defer func() {
		must(counters.Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	producer, err := counters.Producer().Register(ctx, nil)
	must(err)
	janitor, err := janitorcontroller.NewJanitorController(ds, ds.Logger)
	must(err)

	firstWritesCompose(ctx, client, ds, producer, newLabKey(counters, "first-write"), &sqlstreams.CompactionOptions{Enable: true})
	ordinaryProduceFillsEmptyRow(ctx, client, ds, producer, newLabKey(counters, "ordinary-fill"), &sqlstreams.CompactionOptions{Enable: true}, registered.Id)
	ttlRemovesOnlyEmptyRows(ctx, client, ds, janitor, counters, registered.Id)
	lockerFirstSkipsWithoutWaiting(ctx, client, ds, janitor, newLabKey(counters, "locker-first"), registered.Id)
	janitorFirstDeletesThenLockerRecreates(ctx, client, ds, janitor, newLabKey(counters, "janitor-first"), registered.Id)

	fmt.Println("\n✅ COMPACTION-HEAD LOCK E2E TEST PASSED")
	return nil
}

func firstWritesCompose(ctx context.Context, client *sqlstreams.Client, ds *iDatastore.PostgresDatastore, producer *sqlstreams.ProducerInstance[Counter], key testKey, compaction *sqlstreams.CompactionOptions) {
	step("two transactions first-increment an absent key to 2")

	firstLocked := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- increment(ctx, client, producer, key, compaction, func() {
			close(firstLocked)
			<-releaseFirst
		})
	}()
	must(waitSignal(firstLocked, time.Second, "first transaction never locked the absent key"))

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- increment(ctx, client, producer, key, compaction, nil)
	}()
	if err := waitForQueryWait(ctx, ds, "compaction.ensureAndLockHead", "Lock", "", time.Second); err != nil {
		close(releaseFirst)
		must(<-firstDone)
		must(<-secondDone)
		die(err.Error())
	}
	close(releaseFirst)
	must(<-firstDone)
	must(<-secondDone)

	head, err := key.handle.CompactionHead(ctx)
	must(err)
	assertInt("composed value", head.Message.Value, 2)
	fmt.Println("  ✓ second transaction waited on the first row lock")
}

func increment(ctx context.Context, client *sqlstreams.Client, producer *sqlstreams.ProducerInstance[Counter], key testKey, compaction *sqlstreams.CompactionOptions, afterLock func()) error {
	return client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
		head, err := key.handle.LockCompactionHead(ctx, tx)
		if err != nil {
			return err
		}
		value := 0
		if head != nil {
			value = head.Message.Value
		}
		if afterLock != nil {
			afterLock()
		}
		_, err = producer.ProduceInTx(ctx, tx, &Counter{Value: value + 1}, &sqlstreams.ProduceOptions{
			MessageKey: key.name,
			Compaction: compaction,
		})
		return err
	})
}

func ordinaryProduceFillsEmptyRow(ctx context.Context, client *sqlstreams.Client, ds *iDatastore.PostgresDatastore, producer *sqlstreams.ProducerInstance[Counter], key testKey, compaction *sqlstreams.CompactionOptions, streamId int64) {
	step("ordinary compacted produce fills an existing null-head row")
	must(lockOnly(ctx, client, key))
	before, exists := readHeadRow(ctx, ds, streamId, key.name)
	assertTrue("null-head row exists before produce", exists && before.MessageId == nil)

	produced, err := producer.Produce(ctx, &Counter{Value: 7}, &sqlstreams.ProduceOptions{
		MessageKey: key.name,
		Compaction: compaction,
	})
	must(err)
	after, exists := readHeadRow(ctx, ds, streamId, key.name)
	assertTrue("row is materialized after ordinary produce", exists && after.MessageId != nil)
	assertInt64("materialized head id", *after.MessageId, produced.Id)
	assertTrue("produce filled the same row", after.CreatedAt.Equal(before.CreatedAt))
}

func ttlRemovesOnlyEmptyRows(ctx context.Context, client *sqlstreams.Client, ds *iDatastore.PostgresDatastore, janitor *janitorcontroller.JanitorController, counters *sqlstreams.StreamHandle[Counter], streamId int64) {
	step("TTL removes a committed lock-only row and preserves materialized heads")
	empty := newLabKey(counters, "ttl-empty")
	must(lockOnly(ctx, client, empty))
	time.Sleep(emptyHeadTTL + 50*time.Millisecond)

	must(janitor.SweepExpiredEmptyCompactionHeads(ctx, streamId, emptyHeadTTL, batchSize))
	_, exists := readHeadRow(ctx, ds, streamId, empty.name)
	assertTrue("expired null-head row was removed", !exists)
	assertMaterialized(ctx, newLabKey(counters, "first-write"), 2)
	assertMaterialized(ctx, newLabKey(counters, "ordinary-fill"), 7)
}

func lockerFirstSkipsWithoutWaiting(ctx context.Context, client *sqlstreams.Client, ds *iDatastore.PostgresDatastore, janitor *janitorcontroller.JanitorController, key testKey, streamId int64) {
	step("locker-first: janitor skips the locked expired row without waiting")
	must(lockOnly(ctx, client, key))
	backdateHead(ctx, ds, streamId, key.name)

	locked := make(chan struct{})
	release := make(chan struct{})
	lockerDone := make(chan error, 1)
	go func() {
		lockerDone <- client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
			head, err := key.handle.LockCompactionHead(ctx, tx)
			if err != nil {
				return err
			}
			if head != nil {
				return errors.New("lock-only row unexpectedly had a head")
			}
			close(locked)
			<-release
			return nil
		})
	}()
	must(waitSignal(locked, time.Second, "locker-first transaction never acquired the row"))

	sweepDone := make(chan error, 1)
	go func() {
		sweepDone <- janitor.SweepExpiredEmptyCompactionHeads(ctx, streamId, emptyHeadTTL, batchSize)
	}()
	select {
	case err := <-sweepDone:
		must(err)
		fmt.Println("  ✓ janitor returned while the locker still held the row")
	case <-time.After(time.Second):
		close(release)
		must(<-lockerDone)
		die("janitor blocked on the locker despite SKIP LOCKED")
	}
	close(release)
	must(<-lockerDone)

	row, exists := readHeadRow(ctx, ds, streamId, key.name)
	assertTrue("locker-first row survived and remains empty", exists && row.MessageId == nil)
}

func janitorFirstDeletesThenLockerRecreates(ctx context.Context, client *sqlstreams.Client, ds *iDatastore.PostgresDatastore, janitor *janitorcontroller.JanitorController, key testKey, streamId int64) {
	step("janitor-first: waiting locker recreates the row after deletion")
	must(lockOnly(ctx, client, key))
	backdateHead(ctx, ds, streamId, key.name)
	removePause := installDeletePause(ctx, ds, streamId)
	defer removePause()

	sweepDone := make(chan error, 1)
	go func() {
		sweepDone <- janitor.SweepExpiredEmptyCompactionHeads(ctx, streamId, emptyHeadTTL, batchSize)
	}()
	must(waitForQueryWait(ctx, ds, "streamjanitor.sweepEmptyCompactionHeadsBatch", "Timeout", "PgSleep", time.Second))

	lockerDone := make(chan error, 1)
	go func() { lockerDone <- lockOnly(ctx, client, key) }()
	must(waitForQueryWait(ctx, ds, "compaction.ensureAndLockHead", "Lock", "", time.Second))
	must(<-sweepDone)
	must(<-lockerDone)

	row, exists := readHeadRow(ctx, ds, streamId, key.name)
	assertTrue("locker recreated the janitor-deleted row", exists && row.MessageId == nil)
}

func lockOnly(ctx context.Context, client *sqlstreams.Client, key testKey) error {
	return client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
		head, err := key.handle.LockCompactionHead(ctx, tx)
		if err != nil {
			return err
		}
		if head != nil {
			return fmt.Errorf("lock-only key %q unexpectedly returned head id=%d", key.name, head.Id)
		}
		return nil
	})
}

func readHeadRow(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, messageKey string) (headRow, bool) {
	var row headRow
	err := ds.Pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT message_id, created_at, updated_at
		FROM %s.%s
		WHERE compaction_key = $1;
	`, ds.Schema, stream.CompactionHeadTable(streamId)), messageKey).Scan(&row.MessageId, &row.CreatedAt, &row.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return headRow{}, false
	}
	must(err)
	return row, true
}

func backdateHead(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, messageKey string) {
	_, err := ds.Pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.%s
		SET updated_at = NOW() - INTERVAL '1 hour'
		WHERE compaction_key = $1 AND message_id IS NULL;
	`, ds.Schema, stream.CompactionHeadTable(streamId)), messageKey)
	must(err)
}

func installDeletePause(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) func() {
	functionName := fmt.Sprintf("compaction_head_delete_pause_%d", streamId)
	triggerName := fmt.Sprintf("compaction_head_delete_pause_%d", streamId)
	_, err := ds.Pool.Exec(ctx, fmt.Sprintf(`
		CREATE FUNCTION %s.%s() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.5);
			RETURN OLD;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER %s
			BEFORE DELETE ON %s.%s
			FOR EACH ROW EXECUTE FUNCTION %s.%s();
	`, ds.Schema, functionName, triggerName, ds.Schema, stream.CompactionHeadTable(streamId), ds.Schema, functionName))
	must(err)
	return func() {
		_, err := ds.Pool.Exec(ctx, fmt.Sprintf(`
			DROP TRIGGER IF EXISTS %s ON %s.%s;
			DROP FUNCTION IF EXISTS %s.%s();
		`, triggerName, ds.Schema, stream.CompactionHeadTable(streamId), ds.Schema, functionName))
		must(err)
	}
}

func waitForQueryWait(ctx context.Context, ds *iDatastore.PostgresDatastore, marker string, waitType string, waitEvent string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		var waiting bool
		err := ds.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE pid <> pg_backend_pid()
					AND state = 'active'
					AND query LIKE '%' || $1 || '%'
					AND wait_event_type = $2
					AND ($3 = '' OR wait_event = $3)
			);
		`, marker, waitType, waitEvent).Scan(&waiting)
		if err != nil {
			return err
		}
		if waiting {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("query %q never reached wait type %q event %q", marker, waitType, waitEvent)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitSignal(signal <-chan struct{}, timeout time.Duration, message string) error {
	select {
	case <-signal:
		return nil
	case <-time.After(timeout):
		return errors.New(message)
	}
}

func assertMaterialized(ctx context.Context, key testKey, want int) {
	head, err := key.handle.CompactionHead(ctx)
	must(err)
	assertInt(fmt.Sprintf("materialized key %q survived", key.name), head.Message.Value, want)
}

func newLabKey(stream *sqlstreams.StreamHandle[Counter], name string) testKey {
	return testKey{name: name, handle: stream.Key(name)}
}

func step(message string) { fmt.Printf("\n--- %s ---\n", message) }
func must(err error) {
	if err != nil {
		die(err.Error())
	}
}
func die(message string) { panic(testFailure{message: message}) }
func assertTrue(label string, got bool) {
	if !got {
		die(label)
	}
	fmt.Printf("  ✓ %s\n", label)
}
func assertInt(label string, got int, want int) {
	if got != want {
		die(fmt.Sprintf("%s: got %d, want %d", label, got, want))
	}
	fmt.Printf("  ✓ %s (%d)\n", label, got)
}
func assertInt64(label string, got int64, want int64) {
	if got != want {
		die(fmt.Sprintf("%s: got %d, want %d", label, got, want))
	}
	fmt.Printf("  ✓ %s (%d)\n", label, got)
}
