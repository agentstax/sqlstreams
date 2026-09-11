package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iCommon "github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	iMetrics "github.com/agentstax/sqlstreams/pkg/metric"
	metricsproducer "github.com/agentstax/sqlstreams/pkg/metric/producer"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
)

const group = "abandonedevents"

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() (err error) {
	defer common.Recover(&err)
	ctx := context.Background()
	run := time.Now().UnixNano()

	pool, err := common.NewPool(ctx, nil)
	common.Must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)
	common.Must(client.System().Register(ctx, nil))

	metricStream, err := client.Stream[sqlstreams.RawPayload](iMetrics.MetricStreamName).Get(ctx)
	common.Must(err)
	if metricStream == nil {
		common.Die("expected __system.metrics to exist after RegisterSystem")
	}

	streamName := fmt.Sprintf("%s.%d", group, run)
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	before := metricRowCount(ctx, ds, metricStream.Id)

	step("driving a hard timeout so one message gets abandoned then self-clears")
	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)
	seed(ctx, wpInstance, 3)

	cfg := &messageconsumer.MessageConsumerConfig{
		BatchLimit:         3,
		QueueSize:          10,
		MessageConcurrency: 3,
		Message:            &iCommon.MessageOptions{Timeout: 300 * time.Millisecond},
		TimeoutGrace:       50 * time.Millisecond,
	}
	consumerDatastore, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)
	g, err := consumerDatastore.RegisterGroup(ctx, tp.Id, group, consume.Beginning())
	common.Must(err)
	owner, err := iCommon.NewConsumerGroupOwner(tp.SystemId, tp.Id, g.Id, g.Name)
	common.Must(err)

	// the abandoned-event producer outlives any one claim -- the events it
	// carries are generated as the consumer shuts down
	abandonedEvents, err := metricsproducer.NewMetricsProducer(ds, &metricsproducer.MetricProducerConfig{SessionFlushRate: 100 * time.Millisecond}, ds.Logger)
	common.Must(err)
	go func() {
		common.Must(abandonedEvents.Run(ctx, group, tp.Name, 1, "abandonedevents-session"))
	}()

	var calls atomic.Int64
	consumerFunc := func(ctx context.Context, work *common.Work) error {
		if calls.Add(1) == 1 {
			time.Sleep(500 * time.Millisecond)
		}
		return nil
	}

	definition, err := messageconsumer.NewMessageConsumerProvisioner(ds, consumerFunc, 1, abandonedEvents, cfg, ds.Logger)
	common.Must(err)
	common.Must(definition.Declare(ctx, owner))

	runProcessUntil(ctx, ds, definition, owner, 5*time.Second, func() bool {
		return calls.Load() == 3
	})

	step("waiting for __system.metrics to see both the abandoned and cleared events")
	var rows []metricRow
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		rows = metricRowsSince(ctx, ds, metricStream.Id, before)
		if len(rows) >= 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(rows) != 2 {
		common.Die(fmt.Sprintf("expected exactly 2 abandoned-routine events on __system.metrics, got %d: %+v", len(rows), rows))
	}

	abandoned, cleared := rows[0], rows[1]
	assertEqual("first event type", string(abandoned.Event.EventType), string(iMetrics.EventAbandoned))
	assertEqual("second event type", string(cleared.Event.EventType), string(iMetrics.EventCleared))
	assertEqual("abandoned event group", abandoned.Event.Group, group)
	assertEqual("abandoned event stream id", fmt.Sprint(abandoned.Event.StreamId), fmt.Sprint(tp.Id))
	assertEqual("abandoned/cleared share the same message id", fmt.Sprint(abandoned.Event.MessageId), fmt.Sprint(cleared.Event.MessageId))
	wantRoutingKey := fmt.Sprintf("abandoned_routine.%d.%s", tp.Id, group)
	assertEqual("abandoned event routing key", abandoned.RoutingKey, wantRoutingKey)
	assertEqual("cleared event routing key", cleared.RoutingKey, wantRoutingKey)
	fmt.Printf("  ✓ abandoned at %s, cleared at %s (self-clear latency %v)\n", abandoned.Event.At, cleared.Event.At, cleared.Event.At.Sub(abandoned.Event.At))

	fmt.Println("\n✅ ABANDONED EVENTS E2E TEST PASSED")
	return nil
}

type metricRow struct {
	Id         int64
	RoutingKey string
	Event      iMetrics.GoRoutineEvent
}

func metricRowCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) int {
	// the session counters flush to the same stream -- only the
	// abandoned-routine events are this e2e test's subject
	var count int
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE routing_key LIKE 'abandoned_routine.%%'`, ds.Schema, stream.MessageLogTable(streamId))).Scan(&count))
	return count
}

func metricRowsSince(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, sinceCount int) []metricRow {
	rows, err := ds.Pool.Query(ctx, fmt.Sprintf(`
		SELECT id, routing_key, payload FROM %s.%s
		WHERE routing_key LIKE 'abandoned_routine.%%'
		ORDER BY id
		OFFSET %d
	`, ds.Schema, stream.MessageLogTable(streamId), sinceCount))
	common.Must(err)
	defer rows.Close()

	var out []metricRow
	for rows.Next() {
		var id int64
		var routingKey *string
		var payload []byte
		common.Must(rows.Scan(&id, &routingKey, &payload))

		var event iMetrics.GoRoutineEvent
		common.Must(json.Unmarshal(payload, &event))

		rk := ""
		if routingKey != nil {
			rk = *routingKey
		}
		out = append(out, metricRow{Id: id, RoutingKey: rk, Event: event})
	}
	common.Must(rows.Err())
	return out
}

func seed(ctx context.Context, wpInstance *sqlstreams.ProducerInstance[common.Work], n int) {
	for range n {
		_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
			return common.NewWork(30, "admin@example.com")
		}, nil)
		common.Must(err)
	}
}

// no manager, so nothing respawns the execution and the e2e test sees exactly one
// consuming life
func runProcessUntil(ctx context.Context, ds *iDatastore.PostgresDatastore, provisioner worker.Provisioner, owner *iCommon.Owner, timeout time.Duration, done func() bool) {
	workers, err := workercontroller.NewWorkerController(ds, ds.Logger)
	common.Must(err)
	row, err := workers.GetWorker(ctx, provisioner.Definition().Name, owner)
	common.Must(err)

	runCtx, cancel := context.WithCancel(ctx)
	execution, err := provisioner.Provision(runCtx, row)
	common.Must(err)

	errCh := make(chan error, 1)
	go func() { errCh <- execution.Run(runCtx) }()

	start := time.Now()
	for !done() {
		if time.Since(start) > timeout {
			cancel()
			common.Die(fmt.Sprintf("timed out waiting for the expected condition, Process returned: %v", <-errCh))
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
		common.Die(fmt.Sprintf("Process returned an unexpected error: %v", err))
	}
}

func assertEqual(label string, got, want string) {
	if got != want {
		common.Die(fmt.Sprintf("%s: got %q, want %q", label, got, want))
	}
	fmt.Printf("  ✓ %s (%s)\n", label, got)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
