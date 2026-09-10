package schedule

import (
	"context"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/.tests/integration/postgres"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/produce"
	producecontroller "github.com/agentstax/sqlstreams/pkg/produce/controller"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	"github.com/agentstax/sqlstreams/pkg/schedule/controller/datastore"
	producerdatastore "github.com/agentstax/sqlstreams/pkg/schedule/producer/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	systemcontroller "github.com/agentstax/sqlstreams/pkg/system/controller"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// scheduleTestMessage is the payload every scheduled test message carries.
type scheduleTestMessage struct {
	Kind string `json:"kind"`
}

func (scheduleTestMessage) SchemaVersion() int { return 1 }

// newScheduleDatastore registers a system and the "orders" stream every
// schedule targets, and returns the schedule datastore with the stream.
func newScheduleDatastore(t testing.TB) (*datastore.ScheduleDatastore, *stream.Stream) {
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
	schedules, err := datastore.NewScheduleDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return schedules, orders
}

// newScheduleProducerDatastore is the schedule producer datastore over the
// same schema.
func newScheduleProducerDatastore(t testing.TB, schedules *datastore.ScheduleDatastore) *producerdatastore.ScheduleProducerDatastore {
	t.Helper()
	producers, err := producerdatastore.NewScheduleProducerDatastore(schedules.Datastore, schedules.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return producers
}

// registerSchedule registers the schedule on the stream under the cron
// expression: parallel, a one-minute timeout, one payload, no metadata.
func registerSchedule(t testing.TB, schedules *datastore.ScheduleDatastore, orders *stream.Stream, name string, expression string) *datastore.ScheduleConfigRow {
	t.Helper()
	parsed, err := schedule.ParseExpression(expression)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := schedules.Register(t.Context(), orders.SystemId, orders.Id, name, parsed, common.ConcurrencyParallel, time.Minute, scheduleTestMessage{Kind: "nightly"}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	return registered
}

// setNextScheduledAt moves the schedule's next scheduled time to now plus
// the offset.
func setNextScheduledAt(t testing.TB, schedules *datastore.ScheduleDatastore, scheduleId int64, offset time.Duration) {
	t.Helper()
	cursors := schedules.Datastore.Schema + ".schedule_cursor"
	if _, err := schedules.Datastore.Pool.Exec(t.Context(), "UPDATE "+cursors+" SET next_scheduled_at = now() + make_interval(secs => $2) WHERE schedule_id = $1", scheduleId, offset.Seconds()); err != nil {
		t.Fatal(err)
	}
}

// rejectCursorInserts makes every new schedule_cursor row fail; NOT VALID
// leaves the rows already there alone.
func rejectCursorInserts(t testing.TB, schedules *datastore.ScheduleDatastore) {
	t.Helper()
	cursors := schedules.Datastore.Schema + ".schedule_cursor"
	if _, err := schedules.Datastore.Pool.Exec(t.Context(), "ALTER TABLE "+cursors+" ADD CONSTRAINT reject_inserts CHECK (false) NOT VALID"); err != nil {
		t.Fatal(err)
	}
}

// registerConsumerGroup registers a consumer group on the stream with the
// binding patterns; no patterns means the group receives every message.
func registerConsumerGroup(t testing.TB, schedules *datastore.ScheduleDatastore, orders *stream.Stream, name string, patterns []string) *consume.Consumer {
	t.Helper()
	consumers, err := consumecontroller.NewConsumeController(schedules.Datastore, schedules.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := consumers.RegisterGroup(t.Context(), orders.Id, name, consume.CursorPosition{})
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) == 0 {
		return registered
	}
	outcome, err := consumers.DeclareBindings(t.Context(), orders.Id, registered.Id, patterns, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if outcome != consume.BindingInstalled {
		t.Fatalf("DeclareBindings(%s, %v) during setup = %s, want installed", name, patterns, outcome)
	}
	return registered
}

// produceScheduleMessage appends one message the way the schedule producer
// does -- the schedule's name as message key and routing key, compacted,
// carrying the scheduled time -- and returns its id.
func produceScheduleMessage(t testing.TB, schedules *datastore.ScheduleDatastore, orders *stream.Stream, name string, scheduledAt time.Time) int64 {
	t.Helper()
	producers, err := producecontroller.NewProduceController(schedules.Datastore, schedules.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	options := produce.ProduceOptions{
		RoutingKey: name,
		MessageKey: name,
		Compaction: &produce.CompactionOptions{Enable: true},
		Message:    &common.MessageOptions{ScheduledAt: scheduledAt},
	}
	appended, err := producers.AppendMessage(t.Context(), orders.Id, orders.PartitionSize, produceScheduleTestMessage, options)
	if err != nil {
		t.Fatal(err)
	}
	return appended.Id
}

// produceScheduleTestMessage is the ProducerFunc every produced test message
// comes from.
func produceScheduleTestMessage(ctx context.Context, tx iDatastore.Tx) (*scheduleTestMessage, error) {
	return &scheduleTestMessage{Kind: "nightly"}, nil
}

// insertDeliveryOutcome writes one delivery_log row for the message under
// the group with the status -- 'success', 'failure', 'expired', 'killed',
// or 'deferred'.
func insertDeliveryOutcome(t testing.TB, schedules *datastore.ScheduleDatastore, orders *stream.Stream, groupId int64, messageId int64, status string) {
	t.Helper()
	logs := schedules.Datastore.Schema + "." + stream.DeliveryLogTable(orders.Id)
	if _, err := schedules.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+logs+" (consumer_group_id, message_id, attempt, status, error) VALUES ($1, $2, 1, $3, '')", groupId, messageId, status); err != nil {
		t.Fatal(err)
	}
}

// holdTransaction opens a transaction that stays open until the test ends or
// the caller commits or rolls it back.
func holdTransaction(t testing.TB, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}
