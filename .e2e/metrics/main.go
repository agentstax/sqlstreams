package main

// Chunk 5 e2e test: TWO independent MessageConsumer instances (simulating two
// processes) share ONE consumer group on ONE stream. Each hard-times-out
// whatever it claims, proving the abandoned/cleared event stream aggregates
// correctly ACROSS processes -- neither instance's in-memory state is ever
// consulted, everything the assertions below read comes back out of the
// shared __system.metrics stream via client.Stream[sqlstreams.RawPayload](...).Metrics, the same read path
// `sqlstreams stream get` renders.
//
// Retention-drop-out (events aging out of the window) is NOT exercised here:
// __system.metrics is a single shared, already-populated stream with a fixed
// 10,000-row partition size (PartitionSize is immutable after creation), so
// forcing a partition boundary in a short-lived e2e test isn't practical without
// either a huge event volume or a second, parallel metrics stream -- neither
// of which this design supports. The read path applies no separate time
// filter of its own (see pkg/metric/controller/datastore/event.go) -- once a
// partition is physically dropped its rows are just gone from every query,
// so there's no additional logic path here that could get that wrong.

import (
	"context"
	"fmt"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"os"
	"sync"
	"time"

	"github.com/agentstax/sqlstreams/e2e/common"
	iCommon "github.com/agentstax/sqlstreams/pkg/common"
	consumermessage "github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer"
	iMetrics "github.com/agentstax/sqlstreams/pkg/metric"
	metricsproducer "github.com/agentstax/sqlstreams/pkg/metric/producer"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
)

const group = "metrics"

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

// testFailure is what die panics with; run recovers it into its error so
// main's deferred cleanup runs on a failed assertion.
type testFailure struct {
	message string
}

func (f testFailure) Error() string {
	return f.message
}

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
	run := time.Now().UnixNano()

	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	must(err)
	ds, err := datastore.NewPostgresDatastore(ctx, pool, nil)
	must(err)

	streamName := fmt.Sprintf("metrics.%d", run)
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{})
	must(err)
	defer func() {
		must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	must(err)
	for range 4 {
		_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
			return common.NewWork(30, "admin@example.com")
		}, nil)
		must(err)
	}

	step("a lock-only key is visible without making the stream compacted")
	emptySnapshot, err := streamMetrics(ctx, client, streamName)
	must(err)
	assertInt64("headless compaction rows before lock", emptySnapshot.CompactionRowsWithoutHead, 0)
	if emptySnapshot.OldestCompactionRowWithoutHeadAge != 0 {
		die(fmt.Sprintf("oldest headless row age before lock = %v, want 0", emptySnapshot.OldestCompactionRowWithoutHeadAge))
	}
	emptyKey := client.Stream[common.Work](tp.Name).Key("lock-only")
	must(client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
		head, err := emptyKey.LockCompactionHead(ctx, tx)
		if err != nil {
			return err
		}
		if head != nil {
			return fmt.Errorf("new lock-only key returned head id=%d", head.Id)
		}
		return nil
	}))
	time.Sleep(10 * time.Millisecond)
	streamSnapshot, err := streamMetrics(ctx, client, streamName)
	must(err)
	if streamSnapshot.Compacted {
		die("lock-only key made Compacted true")
	}
	assertInt64("headless compaction rows", streamSnapshot.CompactionRowsWithoutHead, 1)
	if streamSnapshot.OldestCompactionRowWithoutHeadAge <= 0 {
		die("expected OldestCompactionRowWithoutHeadAge > 0")
	}
	fmt.Printf("  ✓ oldest headless row age (%v)\n", streamSnapshot.OldestCompactionRowWithoutHeadAge)

	gates := newReleaseGates()
	consumerFunc := func(ctx context.Context, work *common.Work) error {
		meta, _ := consumermessage.MetaFromContext(ctx)
		<-gates.wait(meta.Id) // never returns until released -- simulates a stuck goroutine
		return nil
	}

	cfg := &messageconsumer.MessageConsumerConfig{
		BatchLimit:         2,
		QueueSize:          10,
		MessageConcurrency: 2,
		Message:            &iCommon.MessageOptions{Timeout: 300 * time.Millisecond},
		TimeoutGrace:       100 * time.Millisecond,
		QueueMargin:        200 * time.Millisecond,
		RecordMargin:       200 * time.Millisecond,
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	consumerDatastore, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	must(err)
	g, err := consumerDatastore.RegisterGroup(ctx, tp.Id, group, consumermessage.Beginning())
	must(err)
	owner, err := iCommon.NewConsumerGroupOwner(tp.SystemId, tp.Id, g.Id, g.Name)
	must(err)
	workers, err := workercontroller.NewWorkerController(ds, ds.Logger)
	must(err)

	step("two independent consumer processes claim the same group's cursor")
	var wg sync.WaitGroup
	// consumer rows carry no instance target, so both "processes" claim a life
	// of the same row
	startConsumer := func(label string) {
		abandonedEvents, err := metricsproducer.NewMetricsProducer(ds, &metricsproducer.MetricProducerConfig{SessionFlushRate: 100 * time.Millisecond}, ds.Logger)
		must(err)
		go func() { must(abandonedEvents.Run(runCtx, g.Name, tp.Name, 1, label)) }()

		provisioner, err := messageconsumer.NewMessageConsumerProvisioner(ds, consumerFunc, 1, abandonedEvents, cfg, ds.Logger)
		must(err)
		must(provisioner.Declare(runCtx, owner))

		row, err := workers.GetWorker(runCtx, provisioner.Definition().Name, owner)
		must(err)
		execution, err := provisioner.Provision(runCtx, row)
		must(err)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := execution.Run(runCtx); err != nil && runCtx.Err() == nil {
				die(fmt.Sprintf("%s: Run returned %v", label, err))
			}
		}()
	}
	startConsumer("process A")
	startConsumer("process B")

	step("wait for all 4 messages to hard-timeout across both processes")
	must(waitFor(10*time.Second, func() (bool, error) {
		snap, err := streamMetrics(ctx, client, streamName)
		if err != nil || snap == nil || len(snap.Groups) == 0 {
			return false, err
		}
		return snap.Groups[0].AbandonedRoutines.Total == 4, nil
	}))
	snap := mustStreamMetrics(ctx, client, streamName)
	assertInt64("Total abandoned across both processes", snap.Groups[0].AbandonedRoutines.Total, 4)
	assertInt64("Outstanding (nothing cleared yet)", snap.Groups[0].AbandonedRoutines.Outstanding, 4)

	step("release 2 of the 4 -- outstanding falls, self-clear latency becomes measurable")
	gates.release(1)
	gates.release(2)
	must(waitFor(10*time.Second, func() (bool, error) {
		snap := mustStreamMetrics(ctx, client, streamName)
		return snap.Groups[0].AbandonedRoutines.Outstanding == 2, nil
	}))
	snap = mustStreamMetrics(ctx, client, streamName)
	assertInt64("Total unchanged", snap.Groups[0].AbandonedRoutines.Total, 4)
	assertInt64("Outstanding falls to 2", snap.Groups[0].AbandonedRoutines.Outstanding, 2)
	if snap.Groups[0].AbandonedRoutines.SelfClearLatencyAvg <= 0 {
		die("expected SelfClearLatencyAvg > 0 once some events cleared")
	}
	fmt.Printf("  ✓ SelfClearLatencyAvg (%v)\n", snap.Groups[0].AbandonedRoutines.SelfClearLatencyAvg)

	step("client.Stream[sqlstreams.RawPayload](...).Metrics().Snapshot is the live read `sqlstreams stream get` renders -- cursor/exception state came back too")
	fmt.Printf("  ✓ cursor backlog=%d, ready exceptions=%d\n",
		snap.Groups[0].Cursor.Backlog, snap.Groups[0].Exceptions.Ready)

	cancel()
	wg.Wait()

	fmt.Println("\n✅ METRICS E2E TEST PASSED")
	return nil
}

// ---- release gates: lets consumerFunc block per-message until the test says go ----

type releaseGates struct {
	mu    sync.Mutex
	gates map[int64]chan struct{}
}

func newReleaseGates() *releaseGates {
	return &releaseGates{gates: make(map[int64]chan struct{})}
}

func (g *releaseGates) gate(id int64) chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	ch, ok := g.gates[id]
	if !ok {
		ch = make(chan struct{})
		g.gates[id] = ch
	}
	return ch
}

func (g *releaseGates) wait(id int64) <-chan struct{} { return g.gate(id) }
func (g *releaseGates) release(id int64)              { close(g.gate(id)) }

// ---- helpers ----

func streamMetrics(ctx context.Context, client *sqlstreams.Client, name string) (*iMetrics.StreamSnapshot, error) {
	return client.Stream[sqlstreams.RawPayload](name).Metrics().Snapshot(ctx)
}

func mustStreamMetrics(ctx context.Context, client *sqlstreams.Client, name string) *iMetrics.StreamSnapshot {
	snap, err := streamMetrics(ctx, client, name)
	must(err)
	if len(snap.Groups) == 0 {
		die("expected at least one bound group")
	}
	return snap
}

func waitFor(timeout time.Duration, cond func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	for {
		ok, err := cond()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for condition")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func assertInt64(label string, got, want int64) {
	if got != want {
		die(fmt.Sprintf("%s: got %d, want %d", label, got, want))
	}
	fmt.Printf("  ✓ %s (%d)\n", label, got)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
func must(err error) {
	if err != nil {
		die(err.Error())
	}
}
func die(msg string) {
	panic(testFailure{message: msg})
}
