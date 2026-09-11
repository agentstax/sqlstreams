package main

// Metrics collector e2e test: a full-size collection pass under -race -- the
// collectStreams errgroup fans out stream snapshots under StreamConcurrency,
// every fanned-out stream driving singles and per-group ProduceBatch calls
// against ONE ProducerInstance concurrently. Then the
// pipeline's read half: latest values and history through the public handles
// `sqlstreams metric list` / `sqlstreams metric get` use, and a real
// `sqlstreams manager run --metrics-address` process scraped over HTTP.
// Self-seeding (6 streams x 2 groups x 5 messages), self-cleaning; expects
// .bin/sqlstreams built by the justfile recipe.

import (
	"context"
	"fmt"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iCommon "github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/metric/collector"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
)

const (
	databaseURL       = "postgres://example_user:example_password@localhost:5432/example_db"
	streamCount       = 6
	groupsPerStream   = 2
	messagesPerStream = 5
	collectorRate     = 200 * time.Millisecond
	metricsAddress    = "127.0.0.1:19565"
)

var groupMetricNames = []string{
	metric.MetricCursorHead.Name,
	metric.MetricCursorClaimed.Name,
	metric.MetricCursorCommitted.Name,
	metric.MetricCursorBacklog.Name,
	metric.MetricCursorInflight.Name,
	metric.MetricReadyExceptions.Name,
	metric.MetricInflightExceptions.Name,
	metric.MetricDeferredExceptions.Name,
	metric.MetricDeadExceptions.Name,
	metric.MetricOldestUnresolvedAge.Name,
	metric.MetricOpenLeases.Name,
	metric.MetricAbandonedOutstanding.Name,
	metric.MetricAbandonedTotal.Name,
	metric.MetricAbandonedSelfClearLatencyAvg.Name,
}

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

	ds, err := datastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	step("declare the collector rate through the public system config")
	common.Must(client.System().Register(ctx, nil))
	defer func() { common.Must(client.System().Register(ctx, nil)) }()
	system, err := client.System().Get(ctx)
	common.Must(err)
	systemOwner, err := iCommon.NewSystemOwner(system.Id)
	common.Must(err)
	workers, err := workercontroller.NewWorkerController(ds, ds.Logger)
	common.Must(err)
	row, err := workers.GetWorker(ctx, collector.WorkerMetricsCollector, systemOwner)
	common.Must(err)
	collectorId := row.Id
	for _, rate := range []time.Duration{0, 10 * time.Second, collectorRate} {
		common.Must(client.System().Register(ctx, &sqlstreams.SystemConfig{
			MetricCollector: &sqlstreams.MetricCollectorWorkerConfig{PollRate: rate},
		}))
		row, err = workers.GetWorker(ctx, collector.WorkerMetricsCollector, systemOwner)
		common.Must(err)
		stored, err := workercontroller.ParseMetadata[map[string]time.Duration](row.Metadata)
		common.Must(err)
		expectedRate := rate
		if expectedRate == 0 {
			expectedRate = 30 * time.Second
		}
		if row.Id != collectorId || (*stored)["poll_rate"] != expectedRate {
			common.Die("collector declaration changed its identity or stored the wrong rate")
		}
	}
	err = client.System().Register(ctx, &sqlstreams.SystemConfig{
		MetricCollector: &sqlstreams.MetricCollectorWorkerConfig{PollRate: -time.Second},
	})
	if err == nil || !strings.Contains(err.Error(), "MetricCollector: PollRate") {
		common.Die("negative collector rate did not report its config field")
	}
	row, err = workers.GetWorker(ctx, collector.WorkerMetricsCollector, systemOwner)
	common.Must(err)
	stored, err := workercontroller.ParseMetadata[map[string]time.Duration](row.Metadata)
	common.Must(err)
	if (*stored)["poll_rate"] != collectorRate {
		common.Die("rejected collector rate changed stored metadata")
	}

	step("seed 6 streams x 2 groups x 5 messages -- more streams than StreamConcurrency")
	consumers, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)

	streamNames := make([]string, 0, streamCount)
	groupNames := make([]string, 0, groupsPerStream)
	for g := range groupsPerStream {
		groupNames = append(groupNames, fmt.Sprintf("metricscollector.%c", 'a'+g))
	}
	for t := range streamCount {
		name := fmt.Sprintf("metricscollector.%d.%d", run, t)
		registered, err := client.Stream[common.Work](name).Register(ctx, &sqlstreams.StreamConfig{})
		common.Must(err)
		streamNames = append(streamNames, name)
		defer func() {
			common.Must(client.Stream[common.Work](name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
		}()

		for _, group := range groupNames {
			_, err := consumers.RegisterGroup(ctx, registered.Id, group, consume.Beginning())
			common.Must(err)
		}

		instance, err := client.Stream[common.Work](name).Producer().Register(ctx, nil)
		common.Must(err)
		for range messagesPerStream {
			work, err := common.NewWork(30, "admin@example.com")
			common.Must(err)
			_, err = instance.Produce(ctx, work, nil)
			common.Must(err)
		}
	}

	step("an unpublished series has no latest value or history")
	missingSeries := client.System().Metrics().Metric(
		"metricscollector.unpublished",
		map[string]string{"run": fmt.Sprint(run)},
	)
	latest, err := missingSeries.Latest(ctx)
	common.Must(err)
	if latest != nil {
		common.Die("unpublished series returned a latest measurement")
	}
	history, err := missingSeries.History(ctx, 10)
	common.Must(err)
	if history == nil || len(history) != 0 {
		common.Die("unpublished series did not return an empty history")
	}
	_, err = missingSeries.History(ctx, 0)
	if err == nil {
		common.Die("history accepted a non-positive limit")
	}
	fmt.Println("  ✓ Latest is nil, History is empty, and limit must be positive")

	step("claim the real metrics_collector worker at a fast poll rate")
	row, err = workers.GetWorker(ctx, collector.WorkerMetricsCollector, systemOwner)
	common.Must(err)

	provisioner, err := collector.NewMetricsCollectorProvisioner(ds, &collector.MetricCollectorConfig{
		StreamConcurrency: 4,
	}, ds.Logger)
	common.Must(err)

	// a crashed earlier run's claim lingers until its InstanceTTL expires --
	// retry past it instead of dying
	var execution worker.Execution
	deadline := time.Now().Add(60 * time.Second)
	for {
		execution, err = provisioner.Provision(ctx, row)
		common.Must(err)
		if execution != nil {
			break
		}
		if time.Now().After(deadline) {
			common.Die("metrics collector declined the instance for 60s -- is another claimant running?")
		}
		time.Sleep(time.Second)
	}

	runCtx, cancel := context.WithCancel(ctx)
	collectorDone := make(chan error, 1)
	go func() { collectorDone <- execution.Run(runCtx) }()

	step("wait for full head coverage: fleet + schedules + every e2e test stream and group")
	expected := map[string]bool{
		metric.MeasurementKey(metric.MetricUnclaimedWorkers.Name, nil):            false,
		metric.MeasurementKey(metric.MetricOldestUnclaimedAge.Name, nil):          false,
		metric.MeasurementKey(metric.MetricFailingWorkers.Name, nil):              false,
		metric.MeasurementKey(metric.MetricOverdueSchedules.Name, nil):            false,
		metric.MeasurementKey(metric.MetricOldestDueAge.Name, nil):                false,
		metric.MeasurementKey(metric.MetricSuspendedSchedules.Name, nil):          false,
		metric.MeasurementKey(metric.MetricActiveAlerts.Name, nil):                false,
		metric.MeasurementKey(metric.MetricResolvedAlerts.Name, nil):              false,
		metric.MeasurementKey(metric.MetricCollectorCompletedTimestamp.Name, nil): false,
	}
	for _, streamName := range streamNames {
		for _, name := range []string{metric.MetricStreamCompacted.Name, metric.MetricStreamPartitions.Name, metric.MetricStreamUnclaimedWorkers.Name} {
			expected[metric.MeasurementKey(name, map[string]string{"stream": streamName})] = false
		}
		for _, group := range groupNames {
			for _, name := range groupMetricNames {
				expected[metric.MeasurementKey(name, map[string]string{
					"group": group, "stream": streamName,
				})] = false
			}
		}
	}
	var measurements []*metric.Measurement
	common.Must(waitFor(30*time.Second, func() (bool, error) {
		measurements, err = client.System().Metrics().Latest(ctx)
		if err != nil {
			return false, err
		}
		covered := 0
		for _, measurement := range measurements {
			messageKey := metric.MeasurementKey(measurement.Name, measurement.Attributes)
			if _, ok := expected[messageKey]; ok {
				expected[messageKey] = true
			}
		}
		for _, seen := range expected {
			if seen {
				covered++
			}
		}
		return covered == len(expected), nil
	}))
	fmt.Printf("  ✓ all %d expected series present (%d heads total)\n", len(expected), len(measurements))

	step("head values match the seeded state -- nothing consumed yet")
	byKey := make(map[string]*metric.Measurement, len(measurements))
	for _, measurement := range measurements {
		messageKey := metric.MeasurementKey(measurement.Name, measurement.Attributes)
		byKey[messageKey] = measurement
		if measurement.Attributes["stream"] == metric.MetricStreamName &&
			measurement.Name != metric.MetricStreamPartitions.Name && measurement.Name != metric.MetricStreamUnclaimedWorkers.Name {
			common.Die(fmt.Sprintf("measurement %s adds metrics-stream self-observation beyond alert evidence", messageKey))
		}
	}
	for _, streamName := range streamNames {
		assertValue(byKey, metric.MetricStreamPartitions.Name, map[string]string{"stream": streamName}, 1)
		assertValue(byKey, metric.MetricStreamCompacted.Name, map[string]string{
			"stream": streamName,
		}, 0)
		for _, group := range groupNames {
			attributes := map[string]string{"group": group, "stream": streamName}
			assertValue(byKey, metric.MetricCursorHead.Name, attributes, messagesPerStream)
			assertValue(byKey, metric.MetricCursorBacklog.Name, attributes, messagesPerStream)
			assertValue(byKey, metric.MetricCursorClaimed.Name, attributes, 0)
			assertValue(byKey, metric.MetricDeadExceptions.Name, attributes, 0)
		}
	}
	fmt.Printf("  ✓ compacted=0, head=%d, backlog=%d, claimed=0, dead=0 across %d groups\n",
		messagesPerStream, messagesPerStream, streamCount*groupsPerStream)

	step("history accumulates under the head -- one row per collection pass")
	historySeries := client.Stream[common.Work](streamNames[0]).Consumer(groupNames[0]).Metrics().CursorBacklog()
	common.Must(waitFor(10*time.Second, func() (bool, error) {
		history, err := historySeries.History(ctx, 10)
		if err != nil {
			return false, err
		}
		return len(history) >= 2, nil
	}))
	latest, err = historySeries.Latest(ctx)
	common.Must(err)
	if latest == nil || latest.At.IsZero() || latest.Name != metric.MetricCursorBacklog.Name {
		common.Die("typed backlog series did not return its collected measurement")
	}
	fmt.Println("  ✓ typed backlog selector returns Latest and >= 2 retained History values")

	cancel()
	common.Must(<-collectorDone)

	step("sqlstreams manager run --metrics-address serves the heads as Prometheus text")
	manager := exec.Command("./.bin/sqlstreams", "manager", "run",
		"--metrics-address", metricsAddress,
		"--database-url", databaseURL,
	)
	manager.Stderr = os.Stderr
	common.Must(manager.Start())

	var scrape string
	common.Must(waitFor(15*time.Second, func() (bool, error) {
		response, err := http.Get("http://" + metricsAddress + "/metrics")
		if err != nil {
			return false, nil // not listening yet
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return false, err
		}
		scrape = string(body)
		return response.StatusCode == http.StatusOK, nil
	}))

	for _, series := range []string{
		"sqlstreams_worker_state_unclaimed_workers ",
		"sqlstreams_schedule_state_overdue ",
		fmt.Sprintf("sqlstreams_consumer_cursor_backlog{group=%q,stream=%q} %d", groupNames[0], streamNames[0], messagesPerStream),
		fmt.Sprintf("sqlstreams_stream_state_compacted{stream=%q} 0", streamNames[streamCount-1]),
	} {
		if !strings.Contains(scrape, series) {
			common.Die(fmt.Sprintf("scrape missing %q", series))
		}
		fmt.Printf("  ✓ %s\n", strings.TrimRight(series, " "))
	}

	common.Must(manager.Process.Signal(syscall.SIGTERM))
	common.Must(manager.Wait())
	fmt.Println("  ✓ manager process exited cleanly on SIGTERM")

	fmt.Println("\n✅ METRICS COLLECTOR E2E TEST PASSED")
	return nil
}

// ---- helpers ----

func assertValue(byKey map[string]*metric.Measurement, name string, attributes map[string]string, want float64) {
	key := metric.MeasurementKey(name, attributes)
	head, ok := byKey[key]
	if !ok {
		common.Die(fmt.Sprintf("no head for %s", key))
	}
	if head.Value != want {
		common.Die(fmt.Sprintf("%s: got %v, want %v", key, head.Value, want))
	}
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
		time.Sleep(100 * time.Millisecond)
	}
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
