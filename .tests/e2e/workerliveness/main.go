// Command workerliveness proves the worker_liveness alert end to end:
// what a Register-time pass logs, and what the scheduled check publishes and
// resolves.
//
// Sections:
//  1. register-time -- a produce-only process logs SQL0063 naming the stream's
//     unclaimed stream_janitor; with a consumer running, every row on the
//     stream is claimed and the next Register is silent
//  2. scheduled -- with the group's consumer stopped, a run of the
//     worker_liveness job publishes an active alert naming the group's
//     message_consumer; restarting the consumer resolves it on the next run
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/alert/compactionreadcost"
	"github.com/agentstax/sqlstreams/pkg/alert/partitioncount"
	"github.com/agentstax/sqlstreams/pkg/alert/workerliveness"
	iCommon "github.com/agentstax/sqlstreams/pkg/common"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/metric/collector"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
)

const eventCode = "SQL0063"

// testMessage is the e2e test stream's payload -- the group never has to process
// one, the e2e test only needs the group's worker rows to exist.
type testMessage struct {
	Value string
}

func (testMessage) SchemaVersion() int { return 1 }

var (
	ds     *iDatastore.PostgresDatastore
	client *sqlstreams.Client

	// registerClient logs through capture so the Register-time pass can be counted
	registerClient *sqlstreams.Client
	capture        *captureLogger

	schedulesStream *stream.Stream
	alertsStream    *stream.Stream
	prefix          string

	testStream      *stream.Stream
	testStreamOwner *iCommon.Owner
	testGroupName   string

	jobGroup      int64
	jobGroupOwner *iCommon.Owner
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() (err error) {
	defer common.Recover(&err)
	ctx := context.Background()

	pool, err := common.NewPool(ctx, nil)
	common.Must(err)
	defer pool.Close()

	client, err = sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err = iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)
	common.Must(client.System().Register(ctx, &sqlstreams.SystemConfig{
		WorkerLivenessAlert: &alert.WorkerLivenessAlertConfig{DisablePending: true},
		MetricCollector:     &metric.MetricCollectorWorkerConfig{PollRate: 200 * time.Millisecond},
	}))
	defer func() { common.Must(client.System().Register(ctx, nil)) }()

	capture = newCaptureLogger()
	registerClient, err = sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{Logger: capture})
	common.Must(err)

	schedulesStream, err = client.Stream[sqlstreams.RawPayload](schedule.ScheduleStreamName).Get(ctx)
	common.Must(err)
	alertsStream, err = client.Stream[sqlstreams.RawPayload](alert.AlertStreamName).Get(ctx)
	common.Must(err)

	jobGroup = scalarInt64(ctx,
		fmt.Sprintf(`SELECT id FROM %s.consumer_group_config WHERE stream_id = $1 AND name = $2;`, ds.Schema),
		schedulesStream.Id, workerliveness.JobName)
	jobGroupOwner, err = iCommon.NewConsumerGroupOwner(schedulesStream.SystemId, schedulesStream.Id, jobGroup, workerliveness.JobName)
	common.Must(err)

	prefix = fmt.Sprintf("workerliveness.%d", time.Now().UnixNano())
	testGroupName = prefix + ".group"
	testStream, err = client.Stream[sqlstreams.RawPayload](prefix+".stream").Register(ctx, nil)
	common.Must(err)
	testStreamOwner, err = iCommon.NewStreamOwner(testStream.SystemId, testStream.Id, testStream.Name)
	common.Must(err)
	defer cleanup()

	// only the e2e test's run-nows produce job requests (a suspended job still
	// runs on run-now)
	for _, jobName := range []string{partitioncount.JobName, compactionreadcost.JobName, workerliveness.JobName} {
		common.Must(client.Scheduler(jobName).Suspend(ctx))
	}

	stopCollector := startCollector(ctx)
	defer stopCollector()
	waitCollectedWorkers(ctx, false)
	registerSection(ctx)
	scheduledSection(ctx)

	fmt.Println("\n✅ WORKER LIVENESS E2E TEST PASSED")
	fmt.Println("   a produce-only process learns nothing is running its stream's rows;")
	fmt.Println("   the scheduled check turns the same fact into an alert that resolves itself")
	return nil
}

func registerSection(ctx context.Context) {
	step("register-time: produce-only warns SQL0063, a running consumer silences it")

	// a fresh stream's only worker row is its janitor, and nothing has claimed it
	_, err := registerClient.Stream[testMessage](testStream.Name).Producer().Register(ctx, nil)
	common.Must(err)
	lines := capture.find(eventCode, alert.AlertWorkerLiveness.Name, testStream.Name)
	if len(lines) != 1 {
		common.Die(fmt.Sprintf("produce-only Register: want 1 %s line, got %d", eventCode, len(lines)))
	}
	if detail := lines[0]["detail"]; !strings.Contains(fmt.Sprint(detail), "stream_janitor") {
		common.Die(fmt.Sprintf("produce-only Register: the line must name the unclaimed janitor, got %v", detail))
	}
	fmt.Println("  ✓ a produce-only Register warned once, naming the unclaimed stream_janitor")

	// the consumer's manager claims the stream's rows, its own included
	stopConsumer := startConsumer(ctx)
	waitUnclaimed(ctx, 0)
	waitCollectedWorkers(ctx, true)

	before := len(capture.find(eventCode, alert.AlertWorkerLiveness.Name, testStream.Name))
	_, err = registerClient.Stream[testMessage](testStream.Name).Producer().Register(ctx, nil)
	common.Must(err)
	if got := len(capture.find(eventCode, alert.AlertWorkerLiveness.Name, testStream.Name)); got != before {
		common.Die(fmt.Sprintf("Register under a live consumer must be silent, got %d lines after %d", got, before))
	}
	fmt.Println("  ✓ with every row claimed, the next Register said nothing")

	stopConsumer()
	waitUnclaimed(ctx, 1)
	waitCollectedWorkers(ctx, false)
	fmt.Println("  ✓ stopping the consumer released its rows")
}

func scheduledSection(ctx context.Context) {
	step("scheduled: the check publishes an active alert, a running consumer resolves it")

	stopExecutor := startExecutor(ctx)
	defer stopExecutor()

	activeRun, err := client.Scheduler(workerliveness.JobName).Run(ctx, nil)
	common.Must(err)
	waitDelivered(ctx, activeRun.Id, "success")

	key := alertKey(testStreamOwner)
	if got := headStatus(ctx, key); got != string(alert.AlertStatusActive) {
		common.Die(fmt.Sprintf("the check must publish an active alert for the stream, got %q", got))
	}

	found := listedAlert(ctx)
	if found == nil {
		common.Die("ListAlerts must carry the stream's active worker_liveness alert")
	}
	if !namesWorker(found, "message_consumer", testGroupName) {
		common.Die(fmt.Sprintf("the alert must name the group's unclaimed message_consumer, got %v", found.Data["workers"]))
	}
	fmt.Println("  ✓ the check published an active alert naming the group's message_consumer")

	// every row claimed again -> the same check resolves what it published
	stopConsumer := startConsumer(ctx)
	defer stopConsumer()
	waitUnclaimed(ctx, 0)
	waitCollectedWorkers(ctx, true)

	resolveRun, err := client.Scheduler(workerliveness.JobName).Run(ctx, nil)
	common.Must(err)
	waitDelivered(ctx, resolveRun.Id, "success")
	if got := headStatus(ctx, key); got != string(alert.AlertStatusResolved) {
		common.Die(fmt.Sprintf("a claimed fleet must resolve the alert, got %q", got))
	}
	fmt.Println("  ✓ with the consumer back, the next run resolved it")
}

// --- harness ---

// The collector runs independently so it can observe the e2e test stream without claiming its workers.
func startCollector(ctx context.Context) func() {
	system, err := client.System().Get(ctx)
	common.Must(err)
	owner, err := iCommon.NewSystemOwner(system.Id)
	common.Must(err)
	workers, err := workercontroller.NewWorkerController(ds, ds.Logger)
	common.Must(err)
	row, err := workers.GetWorker(ctx, collector.WorkerMetricsCollector, owner)
	common.Must(err)
	provisioner, err := collector.NewMetricsCollectorProvisioner(ds, nil, ds.Logger)
	common.Must(err)
	execution, err := provisioner.Provision(ctx, row)
	common.Must(err)
	if execution == nil {
		common.Die("metrics collector is already claimed")
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- execution.Run(runCtx) }()
	return func() { cancel(); common.Must(<-done) }
}

func waitCollectedWorkers(ctx context.Context, healthy bool) {
	started := time.Now()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			measurement, err := client.Stream[testMessage](testStream.Name).Metrics().UnclaimedWorkers().Latest(ctx)
			common.Must(err)
			if measurement != nil && measurement.At.After(started) && (measurement.Value == 0) == healthy {
				return
			}
		case <-deadline.C:
			common.Die("collector did not observe the expected worker state within 10s")
		case <-ctx.Done():
			common.Must(ctx.Err())
		}
	}
}

// startConsumer runs a consumer on the e2e test stream until the returned stop is
// called; its manager claims every worker row the stream owns.
func startConsumer(ctx context.Context) func() {
	instance, err := client.Stream[testMessage](testStream.Name).Consumer(testGroupName).Register(ctx, nil)
	common.Must(err)

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- instance.Consume(runCtx, func(ctx context.Context, message *testMessage) error { return nil }, nil)
	}()
	return func() {
		cancel()
		common.Must(<-done)
	}
}

// startExecutor claims the worker_liveness worker row and runs its execution
// until the returned stop is called.
func startExecutor(ctx context.Context) func() {
	provisioner, err := workerliveness.NewWorkerLivenessProvisioner(ds, nil, ds.Logger)
	common.Must(err)
	workers, err := workercontroller.NewWorkerController(ds, ds.Logger)
	common.Must(err)
	row, err := workers.GetWorker(ctx, workerliveness.JobName, jobGroupOwner)
	common.Must(err)
	if row == nil {
		common.Die("RegisterSystem must declare the " + workerliveness.JobName + " worker row")
	}

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
			common.Die("the alert worker declined the instance for 60s -- is a daemon already running?")
		}
		time.Sleep(time.Second)
	}

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- execution.Run(runCtx) }()
	return func() {
		cancel()
		common.Must(<-done)
	}
}

func cleanup() {
	ctx := context.Background()

	for _, jobName := range []string{partitioncount.JobName, compactionreadcost.JobName, workerliveness.JobName} {
		common.Must(client.Scheduler(jobName).Unsuspend(ctx))
	}

	// the check evaluates every stream, so a run leaves a head on each one --
	// all of them are this e2e test's, and nothing is left running to resolve them
	pattern := alert.AlertWorkerLiveness.Name + "/%"
	exec(ctx, fmt.Sprintf(`DELETE FROM %s.%s WHERE compaction_key LIKE $1;`, ds.Schema, stream.CompactionHeadTable(alertsStream.Id)), pattern)
	exec(ctx, fmt.Sprintf(`DELETE FROM %s.%s WHERE message_key LIKE $1;`, ds.Schema, stream.MessageLogTable(alertsStream.Id)), pattern)

	common.Must(client.Stream[testMessage](testStream.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
}

// --- assertion helpers ---

// waitUnclaimed returns once the number of the e2e test stream's worker rows with
// no live instance is at least want -- 0 waits for every row claimed.
func waitUnclaimed(ctx context.Context, want int64) {
	sql := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s.worker_config w
		LEFT JOIN %s.consumer_group_config g ON g.id = w.consumer_group_id
		WHERE COALESCE(w.stream_id, g.stream_id) = %d
			AND NOT EXISTS (SELECT 1 FROM %s.worker_instance i WHERE i.worker_id = w.id AND i.expires_at > now());
	`, ds.Schema, ds.Schema, testStream.Id, ds.Schema)

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		unclaimed := scalarInt64(ctx, sql)
		if (want == 0 && unclaimed == 0) || (want > 0 && unclaimed >= want) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	common.Die(fmt.Sprintf("timed out waiting for %d unclaimed worker rows on the e2e test stream", want))
}

// listedAlert is the e2e test stream's worker_liveness alert as the stream's
// alerts handle reads it, nil when the stream has none.
func listedAlert(ctx context.Context) *alert.Alert {
	found, err := client.Stream[sqlstreams.RawPayload](testStream.Name).Alerts().WorkerLiveness().Latest(ctx)
	common.Must(err)
	return found
}

// namesWorker reports whether the alert's evidence carries the worker row
// under the owner that declared it.
func namesWorker(found *alert.Alert, workerName string, ownerName string) bool {
	rows, ok := found.Data["workers"].([]any)
	if !ok {
		return false
	}
	for _, row := range rows {
		fields, ok := row.(map[string]any)
		if ok && fields["worker"] == workerName && fields["owner"] == ownerName {
			return true
		}
	}
	return false
}

func alertKey(owner *iCommon.Owner) string {
	key, err := alert.MessageKey(alert.AlertWorkerLiveness.Name, owner)
	common.Must(err)
	return key
}

// headStatus is "" when the key has no head or its payload carries no status.
func headStatus(ctx context.Context, messageKey string) string {
	sql := fmt.Sprintf(`
		SELECT m.payload->>'status'
		FROM %s.%s h
		JOIN %s.%s m ON m.id = h.message_id
		WHERE h.compaction_key = $1;
	`, ds.Schema, stream.CompactionHeadTable(alertsStream.Id), ds.Schema, stream.MessageLogTable(alertsStream.Id))
	var status *string
	err := ds.Pool.QueryRow(ctx, sql, messageKey).Scan(&status)
	common.Must(err)
	if status == nil {
		return ""
	}
	return *status
}

// waitDelivered returns once the job group's delivery log holds the request
// at the given status.
func waitDelivered(ctx context.Context, messageId int64, status string) {
	sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = '%s';`, ds.Schema, stream.DeliveryLogTable(schedulesStream.Id), jobGroup, messageId, status)

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if scalarInt64(ctx, sql) >= 1 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	common.Die("timed out waiting for: " + sql)
}

func scalarInt64(ctx context.Context, sql string, args ...any) int64 {
	var value int64
	common.Must(ds.Pool.QueryRow(ctx, sql, args...).Scan(&value))
	return value
}

func exec(ctx context.Context, sql string, args ...any) {
	_, err := ds.Pool.Exec(ctx, sql, args...)
	common.Must(err)
}

// --- capture logger ---

// captureLogger records every line so the register-time pass can be counted
// by code, alert, and owner.
type captureLogger struct {
	mu    sync.Mutex
	lines []map[string]any
}

func newCaptureLogger() *captureLogger {
	return &captureLogger{}
}

func (c *captureLogger) record(args []any) {
	fields := map[string]any{}
	for i := 0; i+1 < len(args); i += 2 {
		if key, ok := args[i].(string); ok {
			fields[key] = args[i+1]
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, fields)
}

func (c *captureLogger) DebugContext(ctx context.Context, message string, args ...any) {
	c.record(args)
}

func (c *captureLogger) InfoContext(ctx context.Context, message string, args ...any) {
	c.record(args)
}

func (c *captureLogger) WarnContext(ctx context.Context, message string, args ...any) {
	c.record(args)
}

func (c *captureLogger) ErrorContext(ctx context.Context, message string, args ...any) {
	c.record(args)
}

// find is every line carrying the code, alert, and owner given.
func (c *captureLogger) find(code string, alertName string, ownerName string) []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	var matches []map[string]any
	for _, line := range c.lines {
		if line["code"] == code && line["alert"] == alertName && line["owner"] == ownerName {
			matches = append(matches, line)
		}
	}
	return matches
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
