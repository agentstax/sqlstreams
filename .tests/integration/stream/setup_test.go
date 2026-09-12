package stream

import (
	"context"
	"strconv"
	"testing"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/.tests/integration/postgres"
	"github.com/agentstax/sqlstreams/client"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	keyleasedatastore "github.com/agentstax/sqlstreams/pkg/consume/base/controller/datastore"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/produce"
	producecontroller "github.com/agentstax/sqlstreams/pkg/produce/controller"
	"github.com/agentstax/sqlstreams/pkg/stream"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	streamdatastore "github.com/agentstax/sqlstreams/pkg/stream/controller/datastore"
	janitordatastore "github.com/agentstax/sqlstreams/pkg/stream/janitor/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/system"
	systemcontroller "github.com/agentstax/sqlstreams/pkg/system/controller"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type retentionTestMessage struct{}

func (retentionTestMessage) SchemaVersion() int { return 1 }

func newRetentionJanitor(t testing.TB) (*janitordatastore.JanitorDatastore, *stream.Stream) {
	t.Helper()
	ds := postgres.Start(t)
	client, err := sqlstreams.NewClient(t.Context(), ds.Pool, &sqlstreams.ClientConfig{Schema: ds.Schema})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.System().Register(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	orders := client.Stream[retentionTestMessage]("orders")
	registered, err := orders.Register(t.Context(), &stream.StreamConfig{PartitionSize: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orders.Consumer("processor").Register(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	janitor, err := janitordatastore.NewJanitorDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return janitor, registered
}

func fillRetentionPartitions(t testing.TB, janitor *janitordatastore.JanitorDatastore) {
	t.Helper()
	ds := janitor.Datastore
	client, err := sqlstreams.NewClient(t.Context(), ds.Pool, &sqlstreams.ClientConfig{Schema: ds.Schema})
	if err != nil {
		t.Fatal(err)
	}
	producer, err := client.Stream[retentionTestMessage]("orders").Producer().Register(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 1001 {
		if _, err := producer.Produce(t.Context(), &retentionTestMessage{}, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func seedRetentionKeys(t testing.TB, janitor *janitordatastore.JanitorDatastore, keys string) {
	t.Helper()
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+keys+" (idempotency_key,created_at) VALUES ('00000000-0000-0000-0000-000000000001',now()),('ffffffff-ffff-ffff-ffff-ffffffffffff',now()-interval '3 hours'),('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',now()-interval '2 hours'),('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',now()-interval '2 hours')"); err != nil {
		t.Fatal(err)
	}
}

func seedRetentionMessages(t testing.TB, janitor *janitordatastore.JanitorDatastore, messages string, cursor string) {
	t.Helper()
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+messages+" (id,schema_version,payload,created_at) SELECT id,1,'{}',CASE WHEN id<=4 THEN now()-interval '2 hours' ELSE now() END FROM generate_series(1,10) id"); err != nil {
		t.Fatal(err)
	}
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "UPDATE "+cursor+" SET committed=3"); err != nil {
		t.Fatal(err)
	}
}

// newStreamDatastore registers a system, which creates the catalog tables,
// and returns the stream datastore with the system.
func newStreamDatastore(t testing.TB) (*streamdatastore.StreamDatastore, *system.System) {
	t.Helper()
	ds := postgres.Start(t)
	systems, err := systemcontroller.NewSystemController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := systems.Register(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	streams, err := streamdatastore.NewStreamDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return streams, registered
}

// declaredStream is the row a registration writes for the name at the
// partition size, every other field at StreamConfig's default.
func declaredStream(systemId int64, name string, partitionSize int64) *streamdatastore.StreamConfigRow {
	cfg := (&stream.StreamConfig{PartitionSize: partitionSize}).WithDefaults()
	return &streamdatastore.StreamConfigRow{
		SystemId:                 systemId,
		Name:                     name,
		PartitionSize:            cfg.PartitionSize,
		RetentionTTLNs:           int64(cfg.RetentionTTL),
		AllowDropPastCommitted:   cfg.AllowDropPastCommitted,
		IdempotencyKeyTTLNs:      int64(cfg.IdempotencyKeyTTL),
		EmptyCompactionHeadTTLNs: int64(cfg.EmptyCompactionHeadTTL),
		DeliveryLogMode:          string(cfg.DeliveryLogMode),
	}
}

// rejectMigrationLogInserts makes every new migration_log row fail; NOT
// VALID leaves the rows already there alone.
func rejectMigrationLogInserts(t testing.TB, streams *streamdatastore.StreamDatastore) {
	t.Helper()
	table := streams.Datastore.Schema + ".migration_log"
	if _, err := streams.Datastore.Pool.Exec(t.Context(), "ALTER TABLE "+table+" ADD CONSTRAINT reject_inserts CHECK (false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
}

// newUnregisteredStreamDatastore is the stream datastore over a schema with
// no system tables at all.
func newUnregisteredStreamDatastore(t testing.TB) *streamdatastore.StreamDatastore {
	t.Helper()
	ds := postgres.Start(t)
	streams, err := streamdatastore.NewStreamDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return streams
}

// registerConsumerGroup registers a consumer group on the stream, its cursor
// before any message.
func registerConsumerGroup(t testing.TB, streams *streamdatastore.StreamDatastore, streamId int64, name string) *consume.Consumer {
	t.Helper()
	consumers, err := consumecontroller.NewConsumeController(streams.Datastore, streams.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := consumers.RegisterGroup(t.Context(), streamId, name, consume.CursorPosition{})
	if err != nil {
		t.Fatal(err)
	}
	return registered
}

// produceMessages appends count messages through the produce path, so the
// partitions the ids need are created ahead as a producer would.
func produceMessages(t testing.TB, ds *datastore.PostgresDatastore, streamId int64, partitionSize int64, count int) {
	t.Helper()
	producers, err := producecontroller.NewProduceController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	for range count {
		if _, err := producers.AppendMessage(t.Context(), streamId, partitionSize, produceRetentionTestMessage, produce.ProduceOptions{}); err != nil {
			t.Fatal(err)
		}
	}
}

// produceRetentionTestMessage is the ProducerFunc every produced test message
// comes from.
func produceRetentionTestMessage(ctx context.Context, tx datastore.Tx) (*retentionTestMessage, error) {
	return &retentionTestMessage{}, nil
}

// streamTables is every per-stream table name of the stream, schema-qualified.
func streamTables(streams *streamdatastore.StreamDatastore, streamId int64) []string {
	schema := streams.Datastore.Schema + "."
	return []string{
		schema + stream.MessageLogTable(streamId),
		schema + stream.ExceptionQueueTable(streamId),
		schema + stream.DeliveryLogTable(streamId),
		schema + stream.IdempotencyKeyTable(streamId),
		schema + stream.ConsumerGroupCursorTable(streamId),
		schema + stream.ClaimLeaseTable(streamId),
		schema + stream.MessageKeyLeaseTable(streamId),
		schema + stream.CompactionHeadTable(streamId),
		schema + stream.BindingConfigTable(streamId),
		schema + stream.BindingConfigLogTable(streamId),
	}
}

// newPartitionedJanitor registers a system, a stream at the smallest
// partition size with a "processor" group whose cursor starts before any
// message, and returns the janitor datastore with the stream. Every two
// messages fill one partition, so a handful of produces make several.
func newPartitionedJanitor(t testing.TB) (*janitordatastore.JanitorDatastore, *stream.Stream) {
	t.Helper()
	ds := postgres.Start(t)
	systems, err := systemcontroller.NewSystemController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	system, err := systems.Register(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	streams, err := streamcontroller.NewStreamController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	orders, err := streams.Register(t.Context(), system.Id, "orders", &stream.StreamConfig{PartitionSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	consumers, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := consumers.RegisterGroup(t.Context(), orders.Id, "processor", consume.CursorPosition{}); err != nil {
		t.Fatal(err)
	}
	janitor, err := janitordatastore.NewJanitorDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return janitor, orders
}

// ageMessages moves the created_at of every message in the id range two
// hours into the past, past any one-hour ttl.
func ageMessages(t testing.TB, janitor *janitordatastore.JanitorDatastore, orders *stream.Stream, low int64, high int64) {
	t.Helper()
	messages := janitor.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "UPDATE "+messages+" SET created_at = now() - interval '2 hours' WHERE id BETWEEN $1 AND $2", low, high); err != nil {
		t.Fatal(err)
	}
}

// commitCursor moves the stream's one group's committed cursor to the id.
func commitCursor(t testing.TB, janitor *janitordatastore.JanitorDatastore, orders *stream.Stream, committed int64) {
	t.Helper()
	cursor := janitor.Datastore.Schema + "." + stream.ConsumerGroupCursorTable(orders.Id)
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "UPDATE "+cursor+" SET committed = $1", committed); err != nil {
		t.Fatal(err)
	}
}

// insertDeliveryRows writes one exception_queue row, one delivery_log row,
// and one compaction_head row for the message under the stream's one group
// -- the rows a partition drop or a row sweep must take with the message.
func insertDeliveryRows(t testing.TB, janitor *janitordatastore.JanitorDatastore, orders *stream.Stream, messageId int64) {
	t.Helper()
	schema := janitor.Datastore.Schema
	groups := schema + ".consumer_group_config"
	queue := schema + "." + stream.ExceptionQueueTable(orders.Id)
	logs := schema + "." + stream.DeliveryLogTable(orders.Id)
	heads := schema + "." + stream.CompactionHeadTable(orders.Id)
	ctx := t.Context()
	if _, err := janitor.Datastore.Pool.Exec(ctx, "INSERT INTO "+queue+" (consumer_group_id, message_id, status, concurrency) SELECT id, $2, 'dead', 'parallel' FROM "+groups+" WHERE stream_id = $1", orders.Id, messageId); err != nil {
		t.Fatal(err)
	}
	if _, err := janitor.Datastore.Pool.Exec(ctx, "INSERT INTO "+logs+" (consumer_group_id, message_id, attempt, error) SELECT id, $2, 1, 'handler returned an error' FROM "+groups+" WHERE stream_id = $1", orders.Id, messageId); err != nil {
		t.Fatal(err)
	}
	if _, err := janitor.Datastore.Pool.Exec(ctx, "INSERT INTO "+heads+" (compaction_key, message_id, schema_version, compaction_rank) VALUES ($1, $2, 1, 0)", "order-"+strconv.FormatInt(messageId, 10), messageId); err != nil {
		t.Fatal(err)
	}
}

// listPartitions is the stream's surviving partition numbers through the
// active one, in order. A produce creates the partition after the active one
// in the background, so whether it exists yet is not the test's to see.
func listPartitions(t testing.TB, janitor *janitordatastore.JanitorDatastore, orders *stream.Stream, active int64) []int64 {
	t.Helper()
	parent := janitor.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	prefix := stream.MessageLogTable(orders.Id) + "_"
	var partitions []int64
	if err := janitor.Datastore.Pool.QueryRow(t.Context(), "SELECT COALESCE(array_agg(n ORDER BY n), '{}') FROM (SELECT replace(c.relname, $2, '')::bigint AS n FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid WHERE i.inhparent = to_regclass($1)) p WHERE n <= $3", parent, prefix, active).Scan(&partitions); err != nil {
		t.Fatal(err)
	}
	return partitions
}

// listMessageIds is the ids a per-stream table still holds in its
// message_id column, in order.
func listMessageIds(t testing.TB, janitor *janitordatastore.JanitorDatastore, table string) []int64 {
	t.Helper()
	var ids []int64
	if err := janitor.Datastore.Pool.QueryRow(t.Context(), "SELECT COALESCE(array_agg(message_id ORDER BY message_id), '{}') FROM "+janitor.Datastore.Schema+"."+table).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	return ids
}

// processorGroupId is the id of the stream's one consumer group.
func processorGroupId(t testing.TB, janitor *janitordatastore.JanitorDatastore, orders *stream.Stream) int64 {
	t.Helper()
	groups := janitor.Datastore.Schema + ".consumer_group_config"
	var id int64
	if err := janitor.Datastore.Pool.QueryRow(t.Context(), "SELECT id FROM "+groups+" WHERE stream_id = $1", orders.Id).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// claimKeyLease takes the key's lease for the stream's one group under a
// fresh token, for an hour.
func claimKeyLease(t testing.TB, janitor *janitordatastore.JanitorDatastore, orders *stream.Stream, key string) {
	t.Helper()
	keys, err := keyleasedatastore.NewKeyLeaseDatastore(janitor.Datastore, janitor.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	held, err := keys.Claim(t.Context(), orders.Id, processorGroupId(t, janitor, orders), key, 1, false, common.ConcurrencyExclusive, 0, 0, time.Hour, pgtype.UUID{Bytes: uuid.New(), Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if held.Verdict != keyleasedatastore.KeyLeaseAcquired {
		t.Fatalf("key lease Claim(%s) during setup = %s, want acquired", key, held.Verdict)
	}
}

// expireKeyLease moves the key's lease expiry into the past.
func expireKeyLease(t testing.TB, janitor *janitordatastore.JanitorDatastore, orders *stream.Stream, key string) {
	t.Helper()
	leases := janitor.Datastore.Schema + "." + stream.MessageKeyLeaseTable(orders.Id)
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "UPDATE "+leases+" SET expires_at = now() - interval '1 second' WHERE message_key = $1", key); err != nil {
		t.Fatal(err)
	}
}

// insertEmptyCompactionHead writes a compaction_head row that points at no
// message, last touched age ago.
func insertEmptyCompactionHead(t testing.TB, janitor *janitordatastore.JanitorDatastore, orders *stream.Stream, key string, age time.Duration) {
	t.Helper()
	heads := janitor.Datastore.Schema + "." + stream.CompactionHeadTable(orders.Id)
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+heads+" (compaction_key, updated_at) VALUES ($1, now() - make_interval(secs => $2))", key, age.Seconds()); err != nil {
		t.Fatal(err)
	}
}

// insertCompactionHead writes a compaction_head row pointing at the message,
// last touched age ago.
func insertCompactionHead(t testing.TB, janitor *janitordatastore.JanitorDatastore, orders *stream.Stream, key string, messageId int64, age time.Duration) {
	t.Helper()
	heads := janitor.Datastore.Schema + "." + stream.CompactionHeadTable(orders.Id)
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+heads+" (compaction_key, message_id, schema_version, compaction_rank, updated_at) VALUES ($1, $2, 1, 0, now() - make_interval(secs => $3))", key, messageId, age.Seconds()); err != nil {
		t.Fatal(err)
	}
}

// listKeys is the values a per-stream table holds in the column, in order.
func listKeys(t testing.TB, janitor *janitordatastore.JanitorDatastore, table string, column string) []string {
	t.Helper()
	var keys []string
	if err := janitor.Datastore.Pool.QueryRow(t.Context(), "SELECT COALESCE(array_agg("+column+" ORDER BY "+column+"), '{}') FROM "+janitor.Datastore.Schema+"."+table).Scan(&keys); err != nil {
		t.Fatal(err)
	}
	return keys
}

// holdTransaction opens a transaction that stays open until the test ends or
// the caller rolls it back.
func holdTransaction(t testing.TB, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

// seedMessages fills the stream's log with messages at ids 1 to count. A
// produce burns an id whenever the next partition is not there yet, and the
// create-ahead runs in the background, so the ids a produce loop lands on
// are not stable: the produces make the partitions through the real path,
// then the rows are replaced with dense ids.
func seedMessages(t testing.TB, janitor *janitordatastore.JanitorDatastore, orders *stream.Stream, count int) {
	t.Helper()
	produceMessages(t, janitor.Datastore, orders.Id, orders.PartitionSize, count)
	messages := janitor.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "DELETE FROM "+messages); err != nil {
		t.Fatal(err)
	}
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+messages+" (id, schema_version, payload) SELECT id, 1, '{}' FROM generate_series(1, $1) AS id", count); err != nil {
		t.Fatal(err)
	}
}

// registerLaggingStream registers a second stream in the system with a
// consumer group whose cursor starts before any message and never moves.
func registerLaggingStream(t testing.TB, janitor *janitordatastore.JanitorDatastore, systemId int64, name string) {
	t.Helper()
	streams, err := streamcontroller.NewStreamController(janitor.Datastore, janitor.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := streams.Register(t.Context(), systemId, name, &stream.StreamConfig{PartitionSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	consumers, err := consumecontroller.NewConsumeController(janitor.Datastore, janitor.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := consumers.RegisterGroup(t.Context(), registered.Id, "processor", consume.CursorPosition{}); err != nil {
		t.Fatal(err)
	}
}
