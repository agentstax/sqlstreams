package consume

import (
	"context"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/.tests/integration/postgres"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	keyleasedatastore "github.com/agentstax/sqlstreams/pkg/consume/base/controller/datastore"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	consumedatastore "github.com/agentstax/sqlstreams/pkg/consume/controller/datastore"
	cursordatastore "github.com/agentstax/sqlstreams/pkg/consume/cursoradvancer/controller/datastore"
	deliverydatastore "github.com/agentstax/sqlstreams/pkg/consume/deliveryconsumer/controller/datastore"
	exceptiondatastore "github.com/agentstax/sqlstreams/pkg/consume/exceptionconsumer/controller/datastore"
	janitordatastore "github.com/agentstax/sqlstreams/pkg/consume/janitor/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller/datastore"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	metricdatastore "github.com/agentstax/sqlstreams/pkg/metric/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/produce"
	producecontroller "github.com/agentstax/sqlstreams/pkg/produce/controller"
	"github.com/agentstax/sqlstreams/pkg/stream"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	streamjanitordatastore "github.com/agentstax/sqlstreams/pkg/stream/janitor/controller/datastore"
	systemcontroller "github.com/agentstax/sqlstreams/pkg/system/controller"
	workerdatastore "github.com/agentstax/sqlstreams/pkg/worker/controller/datastore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newMessageConsumerDatastore registers a system, a stream, and a consumer
// group whose cursor starts before any message, and returns the message
// consumer datastore with the group.
func newMessageConsumerDatastore(t testing.TB) (*datastore.MessageConsumerGroupDatastore, *consume.Consumer) {
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
	registered, err := streams.Register(t.Context(), system.Id, "claims", nil)
	if err != nil {
		t.Fatal(err)
	}
	consumers, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := consumers.RegisterGroup(t.Context(), registered.Id, "readers", consume.CursorPosition{})
	if err != nil {
		t.Fatal(err)
	}
	groups, err := datastore.NewMessageConsumerGroupDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return groups, consumer
}

// newExceptionConsumerDatastore is the exception consumer datastore over the
// same schema.
func newExceptionConsumerDatastore(t testing.TB, groups *datastore.MessageConsumerGroupDatastore) *exceptiondatastore.ExceptionConsumerGroupDatastore {
	t.Helper()
	exceptions, err := exceptiondatastore.NewExceptionConsumerGroupDatastore(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return exceptions
}

// newConsumeDatastore is the consume datastore over the same schema.
func newConsumeDatastore(t testing.TB, groups *datastore.MessageConsumerGroupDatastore) *consumedatastore.ConsumeDatastore {
	t.Helper()
	consumers, err := consumedatastore.NewConsumeDatastore(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return consumers
}

// registerConsumer registers another consumer group on the stream, its
// cursor before any message.
func registerConsumer(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer, name string) *consume.Consumer {
	t.Helper()
	consumers, err := consumecontroller.NewConsumeController(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := consumers.RegisterGroup(t.Context(), consumer.StreamId, name, consume.CursorPosition{})
	if err != nil {
		t.Fatal(err)
	}
	return registered
}

// declareLiveInstance declares a worker owned by the consumer group and
// claims one live instance of it.
func declareLiveInstance(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer) {
	t.Helper()
	var systemId int64
	if err := groups.Datastore.Pool.QueryRow(t.Context(), "SELECT system_id FROM "+groups.Datastore.Schema+".stream_config WHERE id = $1", consumer.StreamId).Scan(&systemId); err != nil {
		t.Fatal(err)
	}
	owner, err := common.NewConsumerGroupOwner(systemId, consumer.StreamId, consumer.Id, consumer.Name)
	if err != nil {
		t.Fatal(err)
	}
	workers, err := workerdatastore.NewWorkerDatastore(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := workers.RegisterWorker(t.Context(), "consumer", owner, nil, 1, "consume_test"); err != nil {
		t.Fatal(err)
	}
	declared, err := workers.GetWorker(t.Context(), "consumer", owner)
	if err != nil {
		t.Fatal(err)
	}
	instance, err := workers.ClaimInstance(t.Context(), declared.Id, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if instance == nil {
		t.Fatal("ClaimInstance declined during setup, want a live instance")
	}
}

// newJanitorDatastore is the consume janitor datastore over the same schema.
func newJanitorDatastore(t testing.TB, groups *datastore.MessageConsumerGroupDatastore) *janitordatastore.JanitorDatastore {
	t.Helper()
	janitor, err := janitordatastore.NewJanitorDatastore(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return janitor
}

// newKeyLeaseDatastore is the key lease datastore over the same schema.
func newKeyLeaseDatastore(t testing.TB, groups *datastore.MessageConsumerGroupDatastore) *keyleasedatastore.KeyLeaseDatastore {
	t.Helper()
	keys, err := keyleasedatastore.NewKeyLeaseDatastore(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

// newCursorAdvancerDatastore is the cursor advancer datastore over the same
// schema.
func newCursorAdvancerDatastore(t testing.TB, groups *datastore.MessageConsumerGroupDatastore) *cursordatastore.CursorAdvancerDatastore {
	t.Helper()
	cursors, err := cursordatastore.NewCursorAdvancerDatastore(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return cursors
}

// newMetricDatastore is the metric datastore over the same schema.
func newMetricDatastore(t testing.TB, groups *datastore.MessageConsumerGroupDatastore) *metricdatastore.MetricDatastore {
	t.Helper()
	metrics, err := metricdatastore.NewMetricsDatastore(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return metrics
}

// produceMessages appends count messages to the stream's log.
func produceMessages(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer, count int) {
	t.Helper()
	messages := groups.Datastore.Schema + "." + stream.MessageLogTable(consumer.StreamId)
	for range count {
		if _, err := groups.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+messages+" (schema_version, payload) VALUES (1, '{}')"); err != nil {
			t.Fatal(err)
		}
	}
}

// produceKeyedMessages appends count messages carrying the key to the
// stream's log.
func produceKeyedMessages(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer, key string, count int) {
	t.Helper()
	messages := groups.Datastore.Schema + "." + stream.MessageLogTable(consumer.StreamId)
	for range count {
		if _, err := groups.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+messages+" (schema_version, message_key, payload) VALUES (1, $1, '{}')", key); err != nil {
			t.Fatal(err)
		}
	}
}

// insertKeyedException writes one ready exception row for the keyed message
// under the concurrency policy, with can_run_after offset from now.
func insertKeyedException(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer, messageId int64, key string, concurrency common.ConcurrencyPolicy, canRunAfter time.Duration) {
	t.Helper()
	queue := groups.Datastore.Schema + "." + stream.ExceptionQueueTable(consumer.StreamId)
	if _, err := groups.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+queue+" (consumer_group_id, message_id, status, message_key, concurrency, can_run_after) VALUES ($1, $2, 'ready', $3, $4, now() + make_interval(secs => $5))", consumer.Id, messageId, key, string(concurrency), canRunAfter.Seconds()); err != nil {
		t.Fatal(err)
	}
}

// insertException writes one exception row for the message at the given
// status, with can_run_after and lease_expires_at offset from now.
func insertException(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer, messageId int64, status string, canRunAfter time.Duration, leaseExpires time.Duration) {
	t.Helper()
	queue := groups.Datastore.Schema + "." + stream.ExceptionQueueTable(consumer.StreamId)
	if _, err := groups.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+queue+" (consumer_group_id, message_id, status, concurrency, can_run_after, lease_token, lease_expires_at) VALUES ($1, $2, $3, 'parallel', now() + make_interval(secs => $4), gen_random_uuid(), now() + make_interval(secs => $5))", consumer.Id, messageId, status, canRunAfter.Seconds(), leaseExpires.Seconds()); err != nil {
		t.Fatal(err)
	}
}

// claimRange claims the group's next range and returns it with its lease.
func claimRange(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer) *datastore.ClaimedRange {
	t.Helper()
	claimed, err := groups.ClaimMessagesWithCursor(t.Context(), consumer.StreamId, consumer.Id, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil {
		t.Fatal("ClaimMessagesWithCursor declined during setup, want a range")
	}
	return claimed
}

// holdTransaction opens a transaction that owns a transaction id and stays
// open until the test ends or the caller commits it.
func holdTransaction(t testing.TB, pool *pgxpool.Pool) pgx.Tx {
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

// newDeliveryConsumerDatastore is the delivery consumer datastore over the
// same schema.
func newDeliveryConsumerDatastore(t testing.TB, groups *datastore.MessageConsumerGroupDatastore) *deliverydatastore.DeliveryConsumerGroupDatastore {
	t.Helper()
	deliveries, err := deliverydatastore.NewDeliveryConsumerGroupDatastore(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return deliveries
}

// produceRoutedMessage appends one message carrying the routing key to the
// stream's log; "" leaves the routing key NULL.
func produceRoutedMessage(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer, routingKey string) {
	t.Helper()
	messages := groups.Datastore.Schema + "." + stream.MessageLogTable(consumer.StreamId)
	if _, err := groups.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+messages+" (schema_version, routing_key, payload) VALUES (1, NULLIF($1, ''), '{}')", routingKey); err != nil {
		t.Fatal(err)
	}
}

// produceCompactedMessages appends count compacted messages carrying the key
// at rank 0 and points the key's compaction head at the last of them.
func produceCompactedMessages(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer, key string, count int) {
	t.Helper()
	messages := groups.Datastore.Schema + "." + stream.MessageLogTable(consumer.StreamId)
	heads := groups.Datastore.Schema + "." + stream.CompactionHeadTable(consumer.StreamId)
	for range count {
		var id int64
		if err := groups.Datastore.Pool.QueryRow(t.Context(), "INSERT INTO "+messages+" (schema_version, message_key, compaction_rank, payload) VALUES (1, $1, 0, '{}') RETURNING id", key).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := groups.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+heads+" (compaction_key, message_id, schema_version, compaction_rank) VALUES ($1, $2, 1, 0) ON CONFLICT (compaction_key) DO UPDATE SET message_id = EXCLUDED.message_id", key, id); err != nil {
			t.Fatal(err)
		}
	}
}

// expireClaimLease moves the group's claim lease expiry into the past.
func expireClaimLease(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, consumer *consume.Consumer) {
	t.Helper()
	leases := groups.Datastore.Schema + "." + stream.ClaimLeaseTable(consumer.StreamId)
	if _, err := groups.Datastore.Pool.Exec(t.Context(), "UPDATE "+leases+" SET expires_at = now() - interval '1 second' WHERE consumer_group_id = $1", consumer.Id); err != nil {
		t.Fatal(err)
	}
}

// newPartitionedMessageConsumerDatastore registers a system, a stream at the
// smallest partition size, and a consumer group whose cursor starts before
// any message, and returns the message consumer datastore with the group
// and the stream. Every two messages fill one partition.
func newPartitionedMessageConsumerDatastore(t testing.TB) (*datastore.MessageConsumerGroupDatastore, *consume.Consumer, *stream.Stream) {
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
	registered, err := streams.Register(t.Context(), system.Id, "claims", &stream.StreamConfig{PartitionSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	consumers, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := consumers.RegisterGroup(t.Context(), registered.Id, "readers", consume.CursorPosition{})
	if err != nil {
		t.Fatal(err)
	}
	groups, err := datastore.NewMessageConsumerGroupDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return groups, consumer, registered
}

// seedPartitionedMessages fills the partitioned stream's log with messages
// at ids 1 to count. A produce burns an id whenever the next partition is
// not there yet and the create-ahead runs in the background, so the
// produces make the partitions through the real path, then the rows are
// replaced with dense ids.
func seedPartitionedMessages(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, registered *stream.Stream, count int) {
	t.Helper()
	producers, err := producecontroller.NewProduceController(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	for range count {
		if _, err := producers.AppendMessage(t.Context(), registered.Id, registered.PartitionSize, producePartitionedTestMessage, produce.ProduceOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	messages := groups.Datastore.Schema + "." + stream.MessageLogTable(registered.Id)
	if _, err := groups.Datastore.Pool.Exec(t.Context(), "DELETE FROM "+messages); err != nil {
		t.Fatal(err)
	}
	if _, err := groups.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+messages+" (id, schema_version, payload) SELECT id, 1, '{}' FROM generate_series(1, $1) AS id", count); err != nil {
		t.Fatal(err)
	}
}

// partitionedTestMessage is the payload seedPartitionedMessages produces.
type partitionedTestMessage struct{}

func (partitionedTestMessage) SchemaVersion() int { return 1 }

// producePartitionedTestMessage is the ProducerFunc every seeded message
// comes from.
func producePartitionedTestMessage(ctx context.Context, tx iDatastore.Tx) (*partitionedTestMessage, error) {
	return &partitionedTestMessage{}, nil
}

// dropPartition ages every message in the id range past a one-hour ttl and
// drops the partitions those messages filled, waiving the committed floor.
func dropPartition(t testing.TB, groups *datastore.MessageConsumerGroupDatastore, registered *stream.Stream, low int64, high int64) {
	t.Helper()
	messages := groups.Datastore.Schema + "." + stream.MessageLogTable(registered.Id)
	if _, err := groups.Datastore.Pool.Exec(t.Context(), "UPDATE "+messages+" SET created_at = now() - interval '2 hours' WHERE id BETWEEN $1 AND $2", low, high); err != nil {
		t.Fatal(err)
	}
	janitor, err := streamjanitordatastore.NewJanitorDatastore(groups.Datastore, groups.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := janitor.DropExpiredPartitions(t.Context(), registered.Id, registered.PartitionSize, time.Hour, true, stream.DeliveryLogModeOff); err != nil {
		t.Fatal(err)
	}
}

// messageIds is the ids of claimed messages in claim order.
func messageIds(messages []datastore.MessageLogRow) []int64 {
	ids := make([]int64, len(messages))
	for i, message := range messages {
		ids[i] = message.Id
	}
	return ids
}
