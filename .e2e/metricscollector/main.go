package main

// Metrics collector e2e test: a full-size collection pass under -race -- the
// collectTopics errgroup fans out topic snapshots under TopicConcurrency,
// every fanned-out topic driving singles and per-group ProduceBatch calls
// against ONE ProducerInstance concurrently. Then the
// pipeline's read half: latest values and history through the public handles
// `vulkan metric list` / `vulkan metric get` use, and a real
// `vulkan manager run --metrics-address` process scraped over HTTP.
// Self-seeding (6 topics x 2 groups x 5 messages), self-cleaning; expects
// .bin/vulkan built by the justfile recipe.

import (
	"context"
	"fmt"
	"github.com/agentstax/vulkan/pkg/datastore"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/agentstax/vulkan/e2e/common"
	iCommon "github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/consume"
	consumecontroller "github.com/agentstax/vulkan/pkg/consume/controller"
	"github.com/agentstax/vulkan/pkg/metric"
	"github.com/agentstax/vulkan/pkg/metric/collector"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
	"github.com/agentstax/vulkan/pkg/worker"
	workercontroller "github.com/agentstax/vulkan/pkg/worker/controller"
)

const (
	databaseURL      = "postgres://example_user:example_password@localhost:5432/example_db"
	topicCount       = 6
	groupsPerTopic   = 2
	messagesPerTopic = 5
	collectorRate    = 200 * time.Millisecond
	metricsAddress   = "127.0.0.1:19565"
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

	pool, err := vulkan.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	must(err)
	defer pool.Close()

	client, err := vulkan.NewClient(ctx, pool, &vulkan.ClientConfig{AllowDestroy: true})
	must(err)

	ds, err := datastore.NewPostgresDatastore(ctx, pool, nil)
	must(err)

	step("declare the collector rate through the public system config")
	must(client.System().Register(ctx, nil))
	defer func() { must(client.System().Register(ctx, nil)) }()
	system, err := client.System().Get(ctx)
	must(err)
	systemOwner, err := iCommon.NewSystemOwner(system.Id)
	must(err)
	workers, err := workercontroller.NewWorkerController(ds, ds.Logger)
	must(err)
	row, err := workers.GetWorker(ctx, collector.WorkerMetricsCollector, systemOwner)
	must(err)
	collectorId := row.Id
	for _, rate := range []time.Duration{0, 10 * time.Second, collectorRate} {
		must(client.System().Register(ctx, &vulkan.SystemConfig{
			MetricCollector: &vulkan.MetricCollectorWorkerConfig{PollRate: rate},
		}))
		row, err = workers.GetWorker(ctx, collector.WorkerMetricsCollector, systemOwner)
		must(err)
		stored, err := workercontroller.ParseMetadata[map[string]time.Duration](row.Metadata)
		must(err)
		expectedRate := rate
		if expectedRate == 0 {
			expectedRate = 30 * time.Second
		}
		if row.Id != collectorId || (*stored)["poll_rate"] != expectedRate {
			die("collector declaration changed its identity or stored the wrong rate")
		}
	}
	err = client.System().Register(ctx, &vulkan.SystemConfig{
		MetricCollector: &vulkan.MetricCollectorWorkerConfig{PollRate: -time.Second},
	})
	if err == nil || !strings.Contains(err.Error(), "MetricCollector: PollRate") {
		die("negative collector rate did not report its config field")
	}
	row, err = workers.GetWorker(ctx, collector.WorkerMetricsCollector, systemOwner)
	must(err)
	stored, err := workercontroller.ParseMetadata[map[string]time.Duration](row.Metadata)
	must(err)
	if (*stored)["poll_rate"] != collectorRate {
		die("rejected collector rate changed stored metadata")
	}

	step("seed 6 topics x 2 groups x 5 messages -- more topics than TopicConcurrency")
	consumers, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	must(err)

	topicNames := make([]string, 0, topicCount)
	groupNames := make([]string, 0, groupsPerTopic)
	for g := range groupsPerTopic {
		groupNames = append(groupNames, fmt.Sprintf("metricscollector.%c", 'a'+g))
	}
	for t := range topicCount {
		name := fmt.Sprintf("metricscollector.%d.%d", run, t)
		registered, err := client.Topic[common.Work](name).Register(ctx, &vulkan.TopicConfig{})
		must(err)
		topicNames = append(topicNames, name)
		defer func() {
			must(client.Topic[common.Work](name).Destroy(ctx, &vulkan.DestroyOptions{Force: true}))
		}()

		for _, group := range groupNames {
			_, err := consumers.RegisterGroup(ctx, registered.Id, group, consume.Beginning())
			must(err)
		}

		instance, err := client.Topic[common.Work](name).Producer().Register(ctx, nil)
		must(err)
		for range messagesPerTopic {
			work, err := common.NewWork(30, "admin@example.com")
			must(err)
			_, err = instance.Produce(ctx, work, nil)
			must(err)
		}
	}

	step("an unpublished series has no latest value or history")
	missingSeries := client.System().Metrics().Metric(
		"metricscollector.unpublished",
		map[string]string{"run": fmt.Sprint(run)},
	)
	latest, err := missingSeries.Latest(ctx)
	must(err)
	if latest != nil {
		die("unpublished series returned a latest measurement")
	}
	history, err := missingSeries.History(ctx, 10)
	must(err)
	if history == nil || len(history) != 0 {
		die("unpublished series did not return an empty history")
	}
	_, err = missingSeries.History(ctx, 0)
	if err == nil {
		die("history accepted a non-positive limit")
	}
	fmt.Println("  ✓ Latest is nil, History is empty, and limit must be positive")

	step("claim the real metrics_collector worker at a fast poll rate")
	row, err = workers.GetWorker(ctx, collector.WorkerMetricsCollector, systemOwner)
	must(err)

	provisioner, err := collector.NewMetricsCollectorProvisioner(ds, &collector.MetricCollectorConfig{
		TopicConcurrency: 4,
	}, ds.Logger)
	must(err)

	// a crashed earlier run's claim lingers until its InstanceTTL expires --
	// retry past it instead of dying
	var execution worker.Execution
	deadline := time.Now().Add(60 * time.Second)
	for {
		execution, err = provisioner.Provision(ctx, row)
		must(err)
		if execution != nil {
			break
		}
		if time.Now().After(deadline) {
			die("metrics collector declined the instance for 60s -- is another claimant running?")
		}
		time.Sleep(time.Second)
	}

	runCtx, cancel := context.WithCancel(ctx)
	collectorDone := make(chan error, 1)
	go func() { collectorDone <- execution.Run(runCtx) }()

	step("wait for full head coverage: fleet + schedules + every e2e test topic and group")
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
	for _, topicName := range topicNames {
		for _, name := range []string{metric.MetricTopicCompacted.Name, metric.MetricTopicPartitions.Name, metric.MetricTopicUnclaimedWorkers.Name} {
			expected[metric.MeasurementKey(name, map[string]string{"topic": topicName})] = false
		}
		for _, group := range groupNames {
			for _, name := range groupMetricNames {
				expected[metric.MeasurementKey(name, map[string]string{
					"group": group, "topic": topicName,
				})] = false
			}
		}
	}
	var measurements []*metric.Measurement
	must(waitFor(30*time.Second, func() (bool, error) {
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
		if measurement.Attributes["topic"] == metric.MetricTopicName &&
			measurement.Name != metric.MetricTopicPartitions.Name && measurement.Name != metric.MetricTopicUnclaimedWorkers.Name {
			die(fmt.Sprintf("measurement %s adds metrics-topic self-observation beyond alert evidence", messageKey))
		}
	}
	for _, topicName := range topicNames {
		assertValue(byKey, metric.MetricTopicPartitions.Name, map[string]string{"topic": topicName}, 1)
		assertValue(byKey, metric.MetricTopicCompacted.Name, map[string]string{
			"topic": topicName,
		}, 0)
		for _, group := range groupNames {
			attributes := map[string]string{"group": group, "topic": topicName}
			assertValue(byKey, metric.MetricCursorHead.Name, attributes, messagesPerTopic)
			assertValue(byKey, metric.MetricCursorBacklog.Name, attributes, messagesPerTopic)
			assertValue(byKey, metric.MetricCursorClaimed.Name, attributes, 0)
			assertValue(byKey, metric.MetricDeadExceptions.Name, attributes, 0)
		}
	}
	fmt.Printf("  ✓ compacted=0, head=%d, backlog=%d, claimed=0, dead=0 across %d groups\n",
		messagesPerTopic, messagesPerTopic, topicCount*groupsPerTopic)

	step("history accumulates under the head -- one row per collection pass")
	historySeries := client.Topic[common.Work](topicNames[0]).Consumer(groupNames[0]).Metrics().CursorBacklog()
	must(waitFor(10*time.Second, func() (bool, error) {
		history, err := historySeries.History(ctx, 10)
		if err != nil {
			return false, err
		}
		return len(history) >= 2, nil
	}))
	latest, err = historySeries.Latest(ctx)
	must(err)
	if latest == nil || latest.At.IsZero() || latest.Name != metric.MetricCursorBacklog.Name {
		die("typed backlog series did not return its collected measurement")
	}
	fmt.Println("  ✓ typed backlog selector returns Latest and >= 2 retained History values")

	cancel()
	must(<-collectorDone)

	step("vulkan manager run --metrics-address serves the heads as Prometheus text")
	manager := exec.Command("./.bin/vulkan", "manager", "run",
		"--metrics-address", metricsAddress,
		"--database-url", databaseURL,
	)
	manager.Stderr = os.Stderr
	must(manager.Start())

	var scrape string
	must(waitFor(15*time.Second, func() (bool, error) {
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
		"vulkan_worker_state_unclaimed_workers ",
		"vulkan_schedule_state_overdue ",
		fmt.Sprintf("vulkan_consumer_cursor_backlog{group=%q,topic=%q} %d", groupNames[0], topicNames[0], messagesPerTopic),
		fmt.Sprintf("vulkan_topic_state_compacted{topic=%q} 0", topicNames[topicCount-1]),
	} {
		if !strings.Contains(scrape, series) {
			die(fmt.Sprintf("scrape missing %q", series))
		}
		fmt.Printf("  ✓ %s\n", strings.TrimRight(series, " "))
	}

	must(manager.Process.Signal(syscall.SIGTERM))
	must(manager.Wait())
	fmt.Println("  ✓ manager process exited cleanly on SIGTERM")

	fmt.Println("\n✅ METRICS COLLECTOR E2E TEST PASSED")
	return nil
}

// ---- helpers ----

func assertValue(byKey map[string]*metric.Measurement, name string, attributes map[string]string, want float64) {
	key := metric.MeasurementKey(name, attributes)
	head, ok := byKey[key]
	if !ok {
		die(fmt.Sprintf("no head for %s", key))
	}
	if head.Value != want {
		die(fmt.Sprintf("%s: got %v, want %v", key, head.Value, want))
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
func must(err error) {
	if err != nil {
		die(err.Error())
	}
}
func die(msg string) {
	panic(testFailure{message: msg})
}
