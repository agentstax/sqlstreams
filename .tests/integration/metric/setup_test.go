package metric

import (
	"testing"
	"time"

	"github.com/allegedlyreliable/sqlstreams/.tests/integration/postgres"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	consumecontroller "github.com/allegedlyreliable/sqlstreams/pkg/consume/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric/controller/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	streamcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/controller"
	systemcontroller "github.com/allegedlyreliable/sqlstreams/pkg/system/controller"
	workerdatastore "github.com/allegedlyreliable/sqlstreams/pkg/worker/controller/datastore"
)

// newMetricDatastore registers a system and the "orders" stream every read
// is about, and returns the metric datastore with the stream.
func newMetricDatastore(t testing.TB) (*datastore.MetricDatastore, *stream.Stream) {
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
	orders, err := streams.Register(t.Context(), system.Id, "orders", nil)
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := datastore.NewMetricsDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return metrics, orders
}

// registerConsumerGroup registers a consumer group on the stream, its cursor
// before any message.
func registerConsumerGroup(t testing.TB, metrics *datastore.MetricDatastore, orders *stream.Stream, name string) *consume.Consumer {
	t.Helper()
	consumers, err := consumecontroller.NewConsumeController(metrics.Datastore, metrics.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := consumers.RegisterGroup(t.Context(), orders.Id, name, consume.CursorPosition{})
	if err != nil {
		t.Fatal(err)
	}
	return registered
}

// registerMetricsStream registers the __system.metrics stream the event
// reads resolve by name, and returns it.
func registerMetricsStream(t testing.TB, metrics *datastore.MetricDatastore, orders *stream.Stream) *stream.Stream {
	t.Helper()
	streams, err := streamcontroller.NewStreamController(metrics.Datastore, metrics.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := streams.Register(t.Context(), orders.SystemId, metric.MetricStreamName, nil)
	if err != nil {
		t.Fatal(err)
	}
	return registered
}

// insertMessages appends count messages at the schema version to the
// stream's log.
func insertMessages(t testing.TB, metrics *datastore.MetricDatastore, orders *stream.Stream, schemaVersion int, count int) {
	t.Helper()
	messages := metrics.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	for range count {
		if _, err := metrics.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+messages+" (schema_version, payload) VALUES ($1, '{}')", schemaVersion); err != nil {
			t.Fatal(err)
		}
	}
}

// insertException writes one exception row for the message under the group
// at the status, created age ago.
func insertException(t testing.TB, metrics *datastore.MetricDatastore, orders *stream.Stream, groupId int64, messageId int64, status string, age time.Duration) {
	t.Helper()
	queue := metrics.Datastore.Schema + "." + stream.ExceptionQueueTable(orders.Id)
	if _, err := metrics.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+queue+" (consumer_group_id, message_id, status, concurrency, created_at) VALUES ($1, $2, $3, 'parallel', now() - make_interval(secs => $4))", groupId, messageId, status, age.Seconds()); err != nil {
		t.Fatal(err)
	}
}

// insertClaimLease writes one live claim_lease row for the group.
func insertClaimLease(t testing.TB, metrics *datastore.MetricDatastore, orders *stream.Stream, groupId int64) {
	t.Helper()
	leases := metrics.Datastore.Schema + "." + stream.ClaimLeaseTable(orders.Id)
	if _, err := metrics.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+leases+" (consumer_group_id, low, high, expires_at) VALUES ($1, 1, 5, now() + interval '1 minute')", groupId); err != nil {
		t.Fatal(err)
	}
}

// setCursor moves the group's claimed and committed cursor columns.
func setCursor(t testing.TB, metrics *datastore.MetricDatastore, orders *stream.Stream, groupId int64, claimed int64, committed int64) {
	t.Helper()
	cursors := metrics.Datastore.Schema + "." + stream.ConsumerGroupCursorTable(orders.Id)
	if _, err := metrics.Datastore.Pool.Exec(t.Context(), "UPDATE "+cursors+" SET claimed = $2, committed = $3 WHERE consumer_group_id = $1", groupId, claimed, committed); err != nil {
		t.Fatal(err)
	}
}

// insertCompactionHead writes a compaction_head row pointing at the message.
func insertCompactionHead(t testing.TB, metrics *datastore.MetricDatastore, orders *stream.Stream, key string, messageId int64) {
	t.Helper()
	heads := metrics.Datastore.Schema + "." + stream.CompactionHeadTable(orders.Id)
	if _, err := metrics.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+heads+" (compaction_key, message_id, schema_version, compaction_rank) VALUES ($1, $2, 1, 0)", key, messageId); err != nil {
		t.Fatal(err)
	}
}

// insertEmptyCompactionHead writes a compaction_head row that points at no
// message, last touched age ago.
func insertEmptyCompactionHead(t testing.TB, metrics *datastore.MetricDatastore, orders *stream.Stream, key string, age time.Duration) {
	t.Helper()
	heads := metrics.Datastore.Schema + "." + stream.CompactionHeadTable(orders.Id)
	if _, err := metrics.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+heads+" (compaction_key, updated_at) VALUES ($1, now() - make_interval(secs => $2))", key, age.Seconds()); err != nil {
		t.Fatal(err)
	}
}

// newWorkerDatastore is the worker datastore over the same schema.
func newWorkerDatastore(t testing.TB, metrics *datastore.MetricDatastore) *workerdatastore.WorkerDatastore {
	t.Helper()
	workers, err := workerdatastore.NewWorkerDatastore(metrics.Datastore, metrics.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return workers
}

// declareWorker registers one worker row under the owner and returns its id.
func declareWorker(t testing.TB, workers *workerdatastore.WorkerDatastore, owner *common.Owner, name string) int64 {
	t.Helper()
	if err := workers.RegisterWorker(t.Context(), name, owner, nil, 2, "metric_test"); err != nil {
		t.Fatal(err)
	}
	declared, err := workers.GetWorker(t.Context(), name, owner)
	if err != nil {
		t.Fatal(err)
	}
	return declared.Id
}

// claimInstance claims one instance of the worker and returns its row.
func claimInstance(t testing.TB, workers *workerdatastore.WorkerDatastore, workerId int64) *workerdatastore.WorkerInstanceRow {
	t.Helper()
	instance, err := workers.ClaimInstance(t.Context(), workerId, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if instance == nil {
		t.Fatalf("ClaimInstance(%d) declined during setup", workerId)
	}
	return instance
}

// recordFailures records count failures on the instance.
func recordFailures(t testing.TB, workers *workerdatastore.WorkerDatastore, instance *workerdatastore.WorkerInstanceRow, count int) {
	t.Helper()
	for range count {
		if _, err := workers.RecordInstanceFailure(t.Context(), instance.Id, instance.Token.Bytes); err != nil {
			t.Fatal(err)
		}
	}
}

// expireInstance moves the instance's lease expiry into the past.
func expireInstance(t testing.TB, workers *workerdatastore.WorkerDatastore, instanceId int64) {
	t.Helper()
	instances := workers.Datastore.Schema + ".worker_instance"
	if _, err := workers.Datastore.Pool.Exec(t.Context(), "UPDATE "+instances+" SET expires_at = now() - interval '1 second' WHERE id = $1", instanceId); err != nil {
		t.Fatal(err)
	}
}

// insertMetricEvent appends one event message to the metrics stream's log
// under the routing key, shaped as the collector produces it.
func insertMetricEvent(t testing.TB, metrics *datastore.MetricDatastore, metricsStream *stream.Stream, routingKey string, eventType metric.EventType, messageId int64, attempt int, at time.Time) {
	t.Helper()
	messages := metrics.Datastore.Schema + "." + stream.MessageLogTable(metricsStream.Id)
	payload := map[string]any{"type": string(eventType), "message_id": messageId, "attempt": attempt, "at": at}
	if _, err := metrics.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+messages+" (schema_version, routing_key, payload) VALUES (1, $1, $2)", routingKey, payload); err != nil {
		t.Fatal(err)
	}
}
