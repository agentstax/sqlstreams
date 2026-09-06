package main

// Compaction-head lock lifecycle lab: the lockable row serializes the first
// read-modify-write, an ordinary compacted produce fills that same row, and
// the topic janitor removes only idle rows without heads. The final two races
// force each side to win first and prove the janitor never waits on an active
// locker while a lock following a delete safely recreates the row.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/topic"
	janitorcontroller "github.com/agentstax/vulkan/pkg/topic/janitor/controller"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
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

type labKey struct {
	name   string
	handle *vulkan.KeyHandle[Counter]
}

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ LAB FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

type labFailure struct {
	message string
}

func (f labFailure) Error() string { return f.message }

func run() (err error) {
	defer func() {
		switch recovered := recover().(type) {
		case nil:
		case labFailure:
			err = recovered
		default:
			panic(recovered)
		}
	}()

	ctx := context.Background()
	pool, err := vulkan.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	must(err)
	defer pool.Close()

	client, err := vulkan.NewClient(ctx, pool, &vulkan.ClientConfig{AllowDestroy: true})
	must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	must(err)

	topicName := fmt.Sprintf("compactionheadlocklab.%d", time.Now().UnixNano())
	counters := client.Topic[Counter](topicName)
	registered, err := counters.Register(ctx, &vulkan.TopicConfig{
		PartitionSize:          1000,
		EmptyCompactionHeadTTL: emptyHeadTTL,
	})
	must(err)
	defer func() {
		must(counters.Destroy(ctx, &vulkan.DestroyOptions{Force: true}))
	}()

	producer, err := counters.Producer().Register(ctx, nil)
	must(err)
	janitor, err := janitorcontroller.NewJanitorController(ds, ds.Logger)
	must(err)
	compaction, err := vulkan.NewCompactionOptions(0)
	must(err)

	firstWritesCompose(ctx, client, ds, producer, newLabKey(counters, "first-write"), compaction)
	ordinaryProduceFillsEmptyRow(ctx, client, ds, producer, newLabKey(counters, "ordinary-fill"), compaction, registered.Id)
	ttlRemovesOnlyEmptyRows(ctx, client, ds, janitor, counters, registered.Id)
	lockerFirstSkipsWithoutWaiting(ctx, client, ds, janitor, newLabKey(counters, "locker-first"), registered.Id)
	janitorFirstDeletesThenLockerRecreates(ctx, client, ds, janitor, newLabKey(counters, "janitor-first"), registered.Id)

	fmt.Println("\n✅ COMPACTION-HEAD LOCK LAB PASSED")
	return nil
}

func firstWritesCompose(ctx context.Context, client *vulkan.Client, ds *iDatastore.PostgresDatastore, producer *vulkan.ProducerInstance[Counter], key labKey, compaction *vulkan.CompactionOptions) {
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

func increment(ctx context.Context, client *vulkan.Client, producer *vulkan.ProducerInstance[Counter], key labKey, compaction *vulkan.CompactionOptions, afterLock func()) error {
	return client.InTransaction(ctx, func(ctx context.Context, tx vulkan.Tx) error {
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
		_, err = producer.ProduceInTx(ctx, tx, &Counter{Value: value + 1}, &vulkan.ProduceOptions{
			MessageKey: key.name,
			Compaction: compaction,
		})
		return err
	})
}

func ordinaryProduceFillsEmptyRow(ctx context.Context, client *vulkan.Client, ds *iDatastore.PostgresDatastore, producer *vulkan.ProducerInstance[Counter], key labKey, compaction *vulkan.CompactionOptions, topicId int64) {
	step("ordinary compacted produce fills an existing null-head row")
	must(lockOnly(ctx, client, key))
	before, exists := readHeadRow(ctx, ds, topicId, key.name)
	assertTrue("null-head row exists before produce", exists && before.MessageId == nil)

	produced, err := producer.Produce(ctx, &Counter{Value: 7}, &vulkan.ProduceOptions{
		MessageKey: key.name,
		Compaction: compaction,
	})
	must(err)
	after, exists := readHeadRow(ctx, ds, topicId, key.name)
	assertTrue("row is materialized after ordinary produce", exists && after.MessageId != nil)
	assertInt64("materialized head id", *after.MessageId, produced.Id)
	assertTrue("produce filled the same row", after.CreatedAt.Equal(before.CreatedAt))
}

func ttlRemovesOnlyEmptyRows(ctx context.Context, client *vulkan.Client, ds *iDatastore.PostgresDatastore, janitor *janitorcontroller.JanitorController, counters *vulkan.TopicHandle[Counter], topicId int64) {
	step("TTL removes a committed lock-only row and preserves materialized heads")
	empty := newLabKey(counters, "ttl-empty")
	must(lockOnly(ctx, client, empty))
	time.Sleep(emptyHeadTTL + 50*time.Millisecond)

	must(janitor.SweepExpiredEmptyCompactionHeads(ctx, topicId, emptyHeadTTL, batchSize))
	_, exists := readHeadRow(ctx, ds, topicId, empty.name)
	assertTrue("expired null-head row was removed", !exists)
	assertMaterialized(ctx, newLabKey(counters, "first-write"), 2)
	assertMaterialized(ctx, newLabKey(counters, "ordinary-fill"), 7)
}

func lockerFirstSkipsWithoutWaiting(ctx context.Context, client *vulkan.Client, ds *iDatastore.PostgresDatastore, janitor *janitorcontroller.JanitorController, key labKey, topicId int64) {
	step("locker-first: janitor skips the locked expired row without waiting")
	must(lockOnly(ctx, client, key))
	backdateHead(ctx, ds, topicId, key.name)

	locked := make(chan struct{})
	release := make(chan struct{})
	lockerDone := make(chan error, 1)
	go func() {
		lockerDone <- client.InTransaction(ctx, func(ctx context.Context, tx vulkan.Tx) error {
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
		sweepDone <- janitor.SweepExpiredEmptyCompactionHeads(ctx, topicId, emptyHeadTTL, batchSize)
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

	row, exists := readHeadRow(ctx, ds, topicId, key.name)
	assertTrue("locker-first row survived and remains empty", exists && row.MessageId == nil)
}

func janitorFirstDeletesThenLockerRecreates(ctx context.Context, client *vulkan.Client, ds *iDatastore.PostgresDatastore, janitor *janitorcontroller.JanitorController, key labKey, topicId int64) {
	step("janitor-first: waiting locker recreates the row after deletion")
	must(lockOnly(ctx, client, key))
	backdateHead(ctx, ds, topicId, key.name)
	removePause := installDeletePause(ctx, ds, topicId)
	defer removePause()

	sweepDone := make(chan error, 1)
	go func() {
		sweepDone <- janitor.SweepExpiredEmptyCompactionHeads(ctx, topicId, emptyHeadTTL, batchSize)
	}()
	must(waitForQueryWait(ctx, ds, "topicjanitor.sweepEmptyCompactionHeadsBatch", "Timeout", "PgSleep", time.Second))

	lockerDone := make(chan error, 1)
	go func() { lockerDone <- lockOnly(ctx, client, key) }()
	must(waitForQueryWait(ctx, ds, "compaction.ensureAndLockHead", "Lock", "", time.Second))
	must(<-sweepDone)
	must(<-lockerDone)

	row, exists := readHeadRow(ctx, ds, topicId, key.name)
	assertTrue("locker recreated the janitor-deleted row", exists && row.MessageId == nil)
}

func lockOnly(ctx context.Context, client *vulkan.Client, key labKey) error {
	return client.InTransaction(ctx, func(ctx context.Context, tx vulkan.Tx) error {
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

func readHeadRow(ctx context.Context, ds *iDatastore.PostgresDatastore, topicId int64, messageKey string) (headRow, bool) {
	var row headRow
	err := ds.Pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT message_id, created_at, updated_at
		FROM %s.%s
		WHERE compaction_key = $1;
	`, ds.Schema, topic.CompactionHeadTable(topicId)), messageKey).Scan(&row.MessageId, &row.CreatedAt, &row.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return headRow{}, false
	}
	must(err)
	return row, true
}

func backdateHead(ctx context.Context, ds *iDatastore.PostgresDatastore, topicId int64, messageKey string) {
	_, err := ds.Pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.%s
		SET updated_at = NOW() - INTERVAL '1 hour'
		WHERE compaction_key = $1 AND message_id IS NULL;
	`, ds.Schema, topic.CompactionHeadTable(topicId)), messageKey)
	must(err)
}

func installDeletePause(ctx context.Context, ds *iDatastore.PostgresDatastore, topicId int64) func() {
	functionName := fmt.Sprintf("compaction_head_delete_pause_%d", topicId)
	triggerName := fmt.Sprintf("compaction_head_delete_pause_%d", topicId)
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
	`, ds.Schema, functionName, triggerName, ds.Schema, topic.CompactionHeadTable(topicId), ds.Schema, functionName))
	must(err)
	return func() {
		_, err := ds.Pool.Exec(ctx, fmt.Sprintf(`
			DROP TRIGGER IF EXISTS %s ON %s.%s;
			DROP FUNCTION IF EXISTS %s.%s();
		`, triggerName, ds.Schema, topic.CompactionHeadTable(topicId), ds.Schema, functionName))
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

func assertMaterialized(ctx context.Context, key labKey, want int) {
	head, err := key.handle.CompactionHead(ctx)
	must(err)
	assertInt(fmt.Sprintf("materialized key %q survived", key.name), head.Message.Value, want)
}

func newLabKey(topic *vulkan.TopicHandle[Counter], name string) labKey {
	return labKey{name: name, handle: topic.Key(name)}
}

func step(message string) { fmt.Printf("\n--- %s ---\n", message) }
func must(err error) {
	if err != nil {
		die(err.Error())
	}
}
func die(message string) { panic(labFailure{message: message}) }
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
