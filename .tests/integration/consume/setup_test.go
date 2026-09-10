package consume

import (
	"context"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/.tests/integration/postgres"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	cursordatastore "github.com/agentstax/sqlstreams/pkg/consume/cursoradvancer/controller/datastore"
	exceptiondatastore "github.com/agentstax/sqlstreams/pkg/consume/exceptionconsumer/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller/datastore"
	metricdatastore "github.com/agentstax/sqlstreams/pkg/metric/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	systemcontroller "github.com/agentstax/sqlstreams/pkg/system/controller"
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
