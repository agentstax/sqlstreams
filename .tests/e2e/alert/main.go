// Command alert proves the default-alert machinery end to end: what
// RegisterSystem seeds, every classify arm, and the live partition_count
// executor claimed as a real worker.
//
// Sections:
//  1. seeding -- both alert schedules + consumer groups + exact declarations + worker
//     rows exist after RegisterSystem; a declared threshold applies on a
//     re-register and a suspended job survives one
//  2. classify -- driven through AlertController with a 2s repeat: the active
//     edge WARNs once, an unchanged condition publishes nothing, the repeat
//     republish moves the head to a fresh row, a severity change publishes
//     silently, the resolve edge INFOs once, resolved stays silent
//  3. executor -- the real partition_count worker: a threshold-1 run
//     publishes active heads + WARN edges, a second run inside the repeat
//     interval publishes nothing, foreign and bindingless groups on the
//     schedules stream receive nothing
//  4. isolation -- one owner's corrupted head fails its Record while every
//     other stream still resolves; fixing the head lets the retry resolve it
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/alert/compactionreadcost"
	alertcontroller "github.com/agentstax/sqlstreams/pkg/alert/controller"
	"github.com/agentstax/sqlstreams/pkg/alert/partitioncount"
	"github.com/agentstax/sqlstreams/pkg/alert/workerliveness"
	iCommon "github.com/agentstax/sqlstreams/pkg/common"
	compactioncontroller "github.com/agentstax/sqlstreams/pkg/compaction/controller"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	iMetrics "github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/producer"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
	"golang.org/x/sync/errgroup"
)

const (
	testCheckName  = "testcheck"
	classifyRepeat = 2 * time.Second
)

// testMessage is the e2e test stream's payload -- its one write creates the
// stream's first partition, so threshold 1 can trip on it.
type testMessage struct {
	Value string
}

func (testMessage) SchemaVersion() int { return 1 }

var (
	ds     *iDatastore.PostgresDatastore
	client *sqlstreams.Client

	schedulesStream *stream.Stream
	alertsStream    *stream.Stream
	prefix          string

	partitionCountGroup int64
	groupOwner          *iCommon.Owner
	testStream          *stream.Stream
	testStreamOwner     *iCommon.Owner
	executorCapture     *captureLogger
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
	common.Must(client.System().Register(ctx, nil))

	schedulesStream, err = client.Stream[sqlstreams.RawPayload](schedule.ScheduleStreamName).Get(ctx)
	common.Must(err)
	alertsStream, err = client.Stream[sqlstreams.RawPayload](alert.AlertStreamName).Get(ctx)
	common.Must(err)
	if schedulesStream == nil || alertsStream == nil {
		common.Die("RegisterSystem must create the schedules and alerts streams")
	}

	prefix = fmt.Sprintf("alert.%d", time.Now().UnixNano())
	defer cleanup()

	seedingSection(ctx)
	classifySection(ctx)

	// the executor's embedded consumer spawns a schedule producer -- suspend the
	// alert schedules so only the e2e test's run-nows produce requests (a suspended
	// job still runs on run-now)
	common.Must(client.Scheduler(partitioncount.JobName).Suspend(ctx))
	common.Must(client.Scheduler(compactionreadcost.JobName).Suspend(ctx))
	common.Must(client.Scheduler(workerliveness.JobName).Suspend(ctx))

	executorCapture = newCaptureLogger()
	stopExecutor := startExecutor(ctx)
	defer stopExecutor()

	executorSection(ctx)
	isolationSection(ctx)

	fmt.Println("\n✅ ALERT E2E TEST PASSED")
	return nil
}

// --- sections ---

func seedingSection(ctx context.Context) {
	step("seeding: jobs/groups/bindings/workers exist; declared threshold applies, suspended survives")

	partitionCountJob, err := client.Scheduler(partitioncount.JobName).Get(ctx)
	common.Must(err)
	if partitionCountJob == nil {
		common.Die("RegisterSystem must seed the " + partitioncount.JobName + " schedule")
	}
	var seeded alert.JobPayload
	common.Must(json.Unmarshal(partitionCountJob.Payload, &seeded))
	if seeded.Threshold != 0 || partitionCountJob.Concurrency != iCommon.ConcurrencyExclusive {
		common.Die(fmt.Sprintf("seeded job: want threshold 0 + defer, got %d %s", seeded.Threshold, partitionCountJob.Concurrency))
	}
	readCostJob, err := client.Scheduler(compactionreadcost.JobName).Get(ctx)
	common.Must(err)
	if readCostJob == nil {
		common.Die("RegisterSystem must seed the " + compactionreadcost.JobName + " schedule")
	}

	declarations, err := client.System().Bindings(ctx)
	common.Must(err)
	for _, jobName := range []string{partitioncount.JobName, compactionreadcost.JobName} {
		declared := false
		for _, declaration := range declarations {
			if declaration.ConsumerGroupName == jobName && declaration.StreamName == schedule.ScheduleStreamName &&
				declaration.Status == consume.BindingInstalled &&
				len(declaration.Patterns) == 1 && declaration.Patterns[0] == jobName {
				declared = true
			}
		}
		if !declared {
			common.Die("group " + jobName + " must declare exactly its job name at RegisterSystem")
		}
	}

	partitionCountGroup = scalarInt64(ctx,
		fmt.Sprintf(`SELECT id FROM %s.consumer_group_config WHERE stream_id = $1 AND name = $2;`, ds.Schema),
		schedulesStream.Id, partitioncount.JobName)
	groupOwner, err = iCommon.NewConsumerGroupOwner(schedulesStream.SystemId, schedulesStream.Id, partitionCountGroup, partitioncount.JobName)
	common.Must(err)
	workers, err := workercontroller.NewWorkerController(ds, ds.Logger)
	common.Must(err)
	row, err := workers.GetWorker(ctx, partitioncount.JobName, groupOwner)
	common.Must(err)
	if row == nil {
		common.Die("RegisterSystem must declare the " + partitioncount.JobName + " worker row")
	}
	fmt.Println("  ✓ both alert schedules, exact declarations, and the worker row exist")

	// a declared threshold applies on every RegisterSystem, and a suspended
	// alert job stays suspended through one
	common.Must(client.Scheduler(compactionreadcost.JobName).Suspend(ctx))
	declareThreshold(ctx, 7)

	reread, err := client.Scheduler(partitioncount.JobName).Get(ctx)
	common.Must(err)
	var redeclared alert.JobPayload
	common.Must(json.Unmarshal(reread.Payload, &redeclared))
	if redeclared.Threshold != 7 {
		common.Die(fmt.Sprintf("declared threshold must apply on re-register, got %d", redeclared.Threshold))
	}
	readCostJob, err = client.Scheduler(compactionreadcost.JobName).Get(ctx)
	common.Must(err)
	if !readCostJob.Suspended {
		common.Die("a suspended alert schedule must survive re-register")
	}

	declareThreshold(ctx, 0)
	common.Must(client.Scheduler(compactionreadcost.JobName).Unsuspend(ctx))
	fmt.Println("  ✓ declared threshold applied, suspended state survived re-register")
}

// declareThreshold re-declares the partition_count alert at threshold, through
// the same call a user changing it would make.
func declareThreshold(ctx context.Context, threshold int64) {
	common.Must(client.System().Register(ctx, &sqlstreams.SystemConfig{
		PartitionCountAlert: &alert.PartitionCountAlertConfig{Threshold: threshold, DisablePending: true},
		MetricCollector:     &iMetrics.MetricCollectorWorkerConfig{PollRate: 100 * time.Millisecond},
	}))
}

func classifySection(ctx context.Context) {
	step("classify: edge WARN, quiet hold, repeat republish, silent severity change, resolve INFO")

	var err error
	testStream, err = client.Stream[sqlstreams.RawPayload](prefix+".stream").Register(ctx, nil)
	common.Must(err)
	testStreamOwner, err = iCommon.NewStreamOwner(testStream.SystemId, testStream.Id, testStream.Name)
	common.Must(err)

	// the alert controller takes the producer package's instance, not the
	// client's wrapper
	alertProducer, err := producer.NewProducer(ds)
	common.Must(err)
	instance, err := alertProducer.Register[alert.Alert](ctx, alert.AlertStreamName, nil)
	common.Must(err)
	heads, err := compactioncontroller.NewCompactionController(ds, ds.Logger)
	common.Must(err)
	capture := newCaptureLogger()
	alerts, err := alertcontroller.NewAlertController(ctx, instance, ds, heads, classifyRepeat, capture)
	common.Must(err)

	key, err := alert.MessageKey(testCheckName, testStreamOwner)
	common.Must(err)
	found, err := alert.NewAlert(testCheckName, testStreamOwner, alert.AlertStatusActive, alert.AlertSeverityWarn, "testcheck condition holds", time.Now(), nil)
	common.Must(err)

	record := func(found *alert.Alert, want alert.RecordOutcome, arm string) {
		state := alert.AlertEvaluationStateHealthy
		if found != nil {
			state = alert.AlertEvaluationStateActive
		}
		evaluation, err := alert.NewAlertEvaluationSnapshot(state, found, nil)
		common.Must(err)
		outcome, err := alerts.Record(ctx, testCheckName, testStreamOwner, evaluation)
		common.Must(err)
		if outcome != want {
			common.Die(fmt.Sprintf("%s: want outcome %q, got %q", arm, want, outcome))
		}
	}

	// active edge: first publish moves the head and WARNs once
	record(found, alert.RecordOutcomeActive, "active edge")
	if got := alertMessageCount(ctx, key); got != 1 {
		common.Die(fmt.Sprintf("active edge: want 1 published message, got %d", got))
	}
	if got := headStatus(ctx, key); got != string(alert.AlertStatusActive) {
		common.Die(fmt.Sprintf("active edge: want head status active, got %q", got))
	}
	if got := capture.count("warn", testCheckName, testStream.Name); got != 1 {
		common.Die(fmt.Sprintf("active edge: want 1 WARN, got %d", got))
	}
	fmt.Println("  ✓ active edge published the head and WARNed once")

	// unchanged condition inside the repeat interval: nothing publishes
	record(found, alert.RecordOutcomeNothing, "quiet hold")
	if got := alertMessageCount(ctx, key); got != 1 {
		common.Die(fmt.Sprintf("quiet hold: want no republish inside the repeat interval, got %d messages", got))
	}
	fmt.Println("  ✓ unchanged condition inside the interval published nothing")

	// repeat republish: the same alert past the interval republishes
	// silently, moving the head to a fresh row so retention can't sweep a
	// live alert
	firstHead := headId(ctx, key)
	time.Sleep(classifyRepeat + 500*time.Millisecond)
	record(found, alert.RecordOutcomeActive, "repeat")
	if got := alertMessageCount(ctx, key); got != 2 {
		common.Die(fmt.Sprintf("repeat: want a republish past the interval, got %d messages", got))
	}
	if got := headId(ctx, key); got == firstHead {
		common.Die("repeat: the republish must move the head to the fresh row")
	}
	if got := capture.count("warn", testCheckName, testStream.Name); got != 1 {
		common.Die(fmt.Sprintf("repeat: the republish must be silent, got %d WARNs", got))
	}
	fmt.Println("  ✓ repeat republish refreshed the head silently")

	// severity change: publishes immediately (still inside the interval),
	// silently -- the head's stored severity is doctored by direct SQL,
	// bypassing the controller
	exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET payload = jsonb_set(payload, '{severity}', '"e2e-critical"') WHERE id = $1;`, ds.Schema, stream.MessageLogTable(alertsStream.Id)), headId(ctx, key))
	record(found, alert.RecordOutcomeActive, "severity change")
	if got := alertMessageCount(ctx, key); got != 3 {
		common.Die(fmt.Sprintf("severity change: want an immediate republish, got %d messages", got))
	}
	if got := capture.count("warn", testCheckName, testStream.Name); got != 1 {
		common.Die(fmt.Sprintf("severity change: the republish must be silent, got %d WARNs", got))
	}
	fmt.Println("  ✓ severity change republished immediately and silently")

	// resolve edge: a nil finding resolves the head with one INFO
	record(nil, alert.RecordOutcomeResolved, "resolve edge")
	if got := alertMessageCount(ctx, key); got != 4 {
		common.Die(fmt.Sprintf("resolve edge: want a resolve publish, got %d messages", got))
	}
	if got := headStatus(ctx, key); got != string(alert.AlertStatusResolved) {
		common.Die(fmt.Sprintf("resolve edge: want head status resolved, got %q", got))
	}
	if got := capture.count("info", testCheckName, testStream.Name); got != 1 {
		common.Die(fmt.Sprintf("resolve edge: want 1 INFO, got %d", got))
	}

	// resolved head + nil finding: nothing
	record(nil, alert.RecordOutcomeNothing, "resolved + nothing found")
	if got := alertMessageCount(ctx, key); got != 4 {
		common.Die(fmt.Sprintf("resolved + nothing found must publish nothing, got %d messages", got))
	}
	fmt.Println("  ✓ resolve edge INFOed once, resolved head stayed silent")

	concurrentAlerts, err := alertcontroller.NewAlertController(ctx, instance, ds, heads, 4*time.Hour, capture)
	common.Must(err)
	for _, test := range []struct {
		name    string
		finding *alert.Alert
		changed alert.RecordOutcome
		count   int
	}{
		{"concurrent activation", found, alert.RecordOutcomeActive, 1},
		{"concurrent quiet checks", found, alert.RecordOutcomeActive, 0},
		{"concurrent recovery", nil, alert.RecordOutcomeResolved, 1},
	} {
		state := alert.AlertEvaluationStateHealthy
		if test.finding != nil {
			state = alert.AlertEvaluationStateActive
		}
		evaluation, err := alert.NewAlertEvaluationSnapshot(state, test.finding, nil)
		common.Must(err)
		before := alertMessageCount(ctx, key)
		start := make(chan struct{})
		outcomes := make(chan alert.RecordOutcome, 16)
		var routines errgroup.Group
		for range 16 {
			routines.Go(func() error {
				<-start
				outcome, err := concurrentAlerts.Record(ctx, testCheckName, testStreamOwner, evaluation)
				if err != nil {
					return err
				}
				outcomes <- outcome
				return nil
			})
		}
		close(start)
		common.Must(routines.Wait())
		close(outcomes)
		changed := 0
		for outcome := range outcomes {
			if outcome == test.changed {
				changed++
			} else if outcome != alert.RecordOutcomeNothing {
				common.Die(fmt.Sprintf("%s: unexpected outcome %q", test.name, outcome))
			}
		}
		if changed != test.count || alertMessageCount(ctx, key)-before != int64(test.count) {
			common.Die(fmt.Sprintf("%s: want %d transitions, got %d", test.name, test.count, changed))
		}
	}
	if capture.count("warn", testCheckName, testStream.Name) != 2 || capture.count("info", testCheckName, testStream.Name) != 2 {
		common.Die("concurrent checks must log each committed transition once")
	}
	fmt.Println("  ✓ concurrent checks recorded and logged each transition once")

	unencodable, err := alert.NewAlert(testCheckName, testStreamOwner, alert.AlertStatusActive, alert.AlertSeverityWarn, "testcheck condition holds", time.Now(), &alert.AlertOptions{
		Data: map[string]any{"unencodable": make(chan struct{})},
	})
	common.Must(err)
	before := alertMessageCount(ctx, key)
	evaluation, err := alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateActive, unencodable, nil)
	common.Must(err)
	outcome, err := concurrentAlerts.Record(ctx, testCheckName, testStreamOwner, evaluation)
	if err == nil || outcome != "" || alertMessageCount(ctx, key) != before || headStatus(ctx, key) != string(alert.AlertStatusResolved) {
		common.Die("a rejected alert write must leave the resolved head and history unchanged")
	}
	if capture.count("warn", testCheckName, testStream.Name) != 2 || capture.count("info", testCheckName, testStream.Name) != 2 {
		common.Die("a rolled-back alert must not log a transition")
	}
	fmt.Println("  ✓ rejected alert write left no transition or log")
}

func executorSection(ctx context.Context) {
	step("executor: threshold-1 run alerts, repeat-interval run is quiet, foreign groups untouched")

	// one write gives the e2e test stream its first partition
	testInstance, err := client.Stream[testMessage](testStream.Name).Producer().Register(ctx, nil)
	common.Must(err)
	_, err = testInstance.Produce(ctx, &testMessage{Value: "seed"}, nil)
	common.Must(err)
	if observations := partitionObservations(ctx); len(observations) != 0 {
		common.Die("registration-time warnings must not store partition observations")
	}

	otherGroup := registerGroup(ctx, prefix+".other", "some.other.job")
	bindinglessGroup := registerGroup(ctx, prefix+".bindingless")

	declareThreshold(ctx, 1)
	waitForCollector(ctx)

	firstRun, err := client.Scheduler(partitioncount.JobName).Run(ctx, nil)
	common.Must(err)
	waitDelivered(ctx, firstRun.Id, "success")
	if observations := partitionObservations(ctx); len(observations) == 0 || observations[0].Message.Value != 1 {
		common.Die("collector must retain the partition count of 1")
	}

	// the running executor's Register declared the group's set
	declarations, err := client.System().Bindings(ctx)
	common.Must(err)
	declared := false
	for _, declaration := range declarations {
		if declaration.ConsumerGroupName == partitioncount.JobName && declaration.StreamName == schedule.ScheduleStreamName &&
			declaration.Status == consume.BindingInstalled &&
			len(declaration.Patterns) == 1 && declaration.Patterns[0] == partitioncount.JobName {
			declared = true
		}
	}
	if !declared {
		common.Die("the live executor must declare exactly its job name")
	}
	fmt.Println("  ✓ the executor declared exactly its job name")

	testKey := partitionCountKey(testStreamOwner)
	schedulesOwner, err := iCommon.NewStreamOwner(schedulesStream.SystemId, schedulesStream.Id, schedulesStream.Name)
	common.Must(err)
	schedulesKey := partitionCountKey(schedulesOwner)
	if got := headStatus(ctx, testKey); got != string(alert.AlertStatusActive) {
		common.Die(fmt.Sprintf("threshold-1 run: want the e2e test stream's head active, got %q", got))
	}
	if got := headStatus(ctx, schedulesKey); got != string(alert.AlertStatusActive) {
		common.Die(fmt.Sprintf("threshold-1 run: want the schedules stream's head active, got %q", got))
	}
	if got := executorCapture.count("warn", alert.AlertPartitionCount.Name, testStream.Name); got != 1 {
		common.Die(fmt.Sprintf("threshold-1 run: want 1 WARN edge for the e2e test stream, got %d", got))
	}
	fmt.Println("  ✓ threshold-1 run published active heads with WARN edges")

	summary := readCheckSummary(ctx)
	if summary[iMetrics.MetricCheckStreamsEvaluated.Name] < 2 ||
		summary[iMetrics.MetricCheckStreamsFailed.Name] != 0 ||
		summary[iMetrics.MetricCheckPublishedAlerts.Name] != summary[iMetrics.MetricCheckStreamsEvaluated.Name] {
		common.Die(fmt.Sprintf("threshold-1 run: want every evaluated stream published and none failed, got %v", summary))
	}
	fmt.Println("  ✓ check summary: every evaluated stream published, none failed")

	// inside the system's 4h repeat interval the same finding publishes nothing
	published := alertMessageCount(ctx, testKey)
	secondRun, err := client.Scheduler(partitioncount.JobName).Run(ctx, nil)
	common.Must(err)
	waitDelivered(ctx, secondRun.Id, "success")
	if observations := partitionObservations(ctx); len(observations) == 0 || observations[0].Message.Value != 1 {
		common.Die("quiet scheduled check must still have collector evidence")
	}
	if got := alertMessageCount(ctx, testKey); got != published {
		common.Die(fmt.Sprintf("repeat-interval run: want no republish, got %d messages after %d", got, published))
	}
	if got := executorCapture.count("warn", alert.AlertPartitionCount.Name, testStream.Name); got != 1 {
		common.Die(fmt.Sprintf("repeat-interval run: want no new WARN, got %d", got))
	}
	fmt.Println("  ✓ a second run inside the repeat interval published nothing")

	summary = readCheckSummary(ctx)
	if summary[iMetrics.MetricCheckPublishedAlerts.Name] != 0 ||
		summary[iMetrics.MetricCheckStreamsFailed.Name] != 0 {
		common.Die(fmt.Sprintf("repeat-interval run: want a quiet summary, got %v", summary))
	}
	fmt.Println("  ✓ check summary: the quiet run counted zero publishes")

	// exact-name dispatch: the live consumer never claims another job's
	// request, and alert traffic leaves other groups alone
	readCostRun, err := client.Scheduler(compactionreadcost.JobName).Run(ctx, nil)
	common.Must(err)
	thirdRun, err := client.Scheduler(partitioncount.JobName).Run(ctx, nil)
	common.Must(err)
	waitDelivered(ctx, thirdRun.Id, "success")
	if got := scalarInt64(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d;`, ds.Schema, stream.DeliveryLogTable(schedulesStream.Id), partitionCountGroup, readCostRun.Id)); got != 0 {
		common.Die(fmt.Sprintf("the executor must not claim another job's request, got %d delivery rows", got))
	}
	for _, foreignGroup := range []int64{otherGroup, bindinglessGroup} {
		claimed := scalarInt64(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d;`, ds.Schema, stream.ExceptionQueueTable(schedulesStream.Id), foreignGroup))
		logged := scalarInt64(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d;`, ds.Schema, stream.DeliveryLogTable(schedulesStream.Id), foreignGroup))
		if claimed != 0 || logged != 0 {
			common.Die(fmt.Sprintf("group %d must be untouched by alert runs, got %d claims %d log rows", foreignGroup, claimed, logged))
		}
	}
	fmt.Println("  ✓ foreign request unclaimed, other/bindingless groups untouched")
}

func isolationSection(ctx context.Context) {
	step("isolation: a corrupted head fails its stream's Record, the others still resolve")

	testKey := partitionCountKey(testStreamOwner)
	schedulesOwner, err := iCommon.NewStreamOwner(schedulesStream.SystemId, schedulesStream.Id, schedulesStream.Name)
	common.Must(err)
	schedulesKey := partitionCountKey(schedulesOwner)

	// the head row stays, but its payload no longer unmarshals into an Alert
	corruptedHead := headId(ctx, testKey)
	saved := scalarString(ctx, fmt.Sprintf(`SELECT payload::text FROM %s.%s WHERE id = $1;`, ds.Schema, stream.MessageLogTable(alertsStream.Id)), corruptedHead)
	exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET payload = '"corrupt"'::jsonb WHERE id = $1;`, ds.Schema, stream.MessageLogTable(alertsStream.Id)), corruptedHead)

	declareThreshold(ctx, 0)
	resolveRun, err := client.Scheduler(partitioncount.JobName).Run(ctx, nil)
	common.Must(err)

	// the attempt fails on the corrupted owner -- but the same attempt
	// already resolved every healthy stream
	waitDelivered(ctx, resolveRun.Id, "failure")
	if observations := partitionObservations(ctx); len(observations) == 0 || observations[0].Message.Value != 1 {
		common.Die("healthy partition evidence must survive failed alert recording")
	}
	if got := headStatus(ctx, schedulesKey); got != string(alert.AlertStatusResolved) {
		common.Die(fmt.Sprintf("isolation: healthy streams must resolve beside the failure, got %q", got))
	}
	if got := executorCapture.count("info", alert.AlertPartitionCount.Name, schedulesStream.Name); got != 1 {
		common.Die(fmt.Sprintf("isolation: want 1 resolve INFO for the healthy stream, got %d", got))
	}
	if got := headId(ctx, testKey); got != corruptedHead {
		common.Die("isolation: the corrupted owner's head must not move")
	}
	fmt.Println("  ✓ healthy streams resolved in the same attempt the corrupted owner failed")

	// resolved is left unasserted here: an automatic retry may already have
	// overwritten the summary, and only its failed/published counts repeat
	summary := readCheckSummary(ctx)
	if summary[iMetrics.MetricCheckStreamsFailed.Name] != 1 ||
		summary[iMetrics.MetricCheckPublishedAlerts.Name] != 0 {
		common.Die(fmt.Sprintf("isolation: the failed run must still produce its summary, got %v", summary))
	}
	fmt.Println("  ✓ check summary went out on the failed run: exactly 1 stream failed")

	// fixing the head lets the request's retry resolve the last owner
	exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET payload = $1::jsonb WHERE id = $2;`, ds.Schema, stream.MessageLogTable(alertsStream.Id)), saved, corruptedHead)
	waitDelivered(ctx, resolveRun.Id, "success")
	if got := headStatus(ctx, testKey); got != string(alert.AlertStatusResolved) {
		common.Die(fmt.Sprintf("isolation: want the fixed owner resolved on retry, got %q", got))
	}
	if got := executorCapture.count("info", alert.AlertPartitionCount.Name, testStream.Name); got != 1 {
		common.Die(fmt.Sprintf("isolation: want 1 resolve INFO for the fixed owner, got %d", got))
	}
	fmt.Println("  ✓ retry resolved the fixed owner; healthy streams resolved exactly once")

	summary = readCheckSummary(ctx)
	if summary[iMetrics.MetricCheckStreamsFailed.Name] != 0 ||
		summary[iMetrics.MetricCheckResolvedAlerts.Name] != 1 {
		common.Die(fmt.Sprintf("isolation retry: want 0 failed and only the fixed owner resolved, got %v", summary))
	}
	fmt.Println("  ✓ retry summary: zero failed, only the fixed owner resolved")
}

// --- harness ---

func waitForCollector(ctx context.Context) {
	started := time.Now()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			completion, err := client.System().Metrics().CollectorCompletedTimestamp().Latest(ctx)
			common.Must(err)
			if completion != nil && completion.At.After(started) {
				return
			}
		case <-deadline.C:
			common.Die("manager's collector did not complete a pass within 10s")
		case <-ctx.Done():
			common.Must(ctx.Err())
		}
	}
}

func partitionObservations(ctx context.Context) []*iCommon.StoredMessage[iMetrics.Measurement] {
	metricStream, err := client.Stream[iMetrics.Measurement](iMetrics.MetricStreamName).Get(ctx)
	common.Must(err)
	heads, err := compactioncontroller.NewCompactionController(ds, ds.Logger)
	common.Must(err)
	key := iMetrics.MeasurementKey(iMetrics.MetricStreamPartitions.Name, map[string]string{"stream": testStream.Name})
	observations, err := heads.ListKeyMessages[iMetrics.Measurement](ctx, metricStream.Id, key, 100)
	common.Must(err)
	for _, observation := range observations {
		if observation.CompactionRank != 0 || observation.CreatedAt.IsZero() {
			common.Die("partition observations must use default rank and retain their storage timestamp")
		}
	}
	return observations
}

// startExecutor claims the partition_count worker row and runs its execution
// until the returned stop is called.
func startExecutor(ctx context.Context) func() {
	provisioner, err := partitioncount.NewPartitionCountProvisioner(ds, nil, executorCapture)
	common.Must(err)
	workers, err := workercontroller.NewWorkerController(ds, ds.Logger)
	common.Must(err)
	row, err := workers.GetWorker(ctx, partitioncount.JobName, groupOwner)
	common.Must(err)
	if row == nil {
		common.Die("the " + partitioncount.JobName + " worker row is missing")
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

// registerGroup creates a consumer group on the schedules stream, bound to
// the given job names (none = bindingless), and returns its id.
func registerGroup(ctx context.Context, name string, bindings ...string) int64 {
	controller, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)
	group, err := controller.RegisterGroup(ctx, schedulesStream.Id, name, consume.Beginning())
	common.Must(err)
	_, err = controller.DeclareBindings(ctx, schedulesStream.Id, group.Id, bindings, time.Now())
	common.Must(err)
	return group.Id
}

func cleanup() {
	ctx := context.Background()

	common.Must(client.Scheduler(partitioncount.JobName).Unsuspend(ctx))
	common.Must(client.Scheduler(compactionreadcost.JobName).Unsuspend(ctx))

	testKey := partitionCountKey(testStreamOwner)
	checkKey, err := alert.MessageKey(testCheckName, testStreamOwner)
	common.Must(err)
	keys := []string{testKey, checkKey}
	exec(ctx, fmt.Sprintf(`DELETE FROM %s.%s WHERE compaction_key = ANY($1);`, ds.Schema, stream.CompactionHeadTable(alertsStream.Id)), keys)
	exec(ctx, fmt.Sprintf(`DELETE FROM %s.%s WHERE message_key = ANY($1);`, ds.Schema, stream.MessageLogTable(alertsStream.Id)), keys)

	common.Must(client.Stream[testMessage](testStream.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))

	for _, sql := range []string{
		fmt.Sprintf(`DELETE FROM %s.%s WHERE consumer_group_id IN (SELECT id FROM %s.consumer_group_config WHERE name LIKE '%s.%%');`, ds.Schema, stream.ExceptionQueueTable(schedulesStream.Id), ds.Schema, prefix),
		fmt.Sprintf(`DELETE FROM %s.%s WHERE consumer_group_id IN (SELECT id FROM %s.consumer_group_config WHERE name LIKE '%s.%%');`, ds.Schema, stream.DeliveryLogTable(schedulesStream.Id), ds.Schema, prefix),
		fmt.Sprintf(`DELETE FROM %s.%s WHERE consumer_group_id IN (SELECT id FROM %s.consumer_group_config WHERE name LIKE '%s.%%');`, ds.Schema, stream.ClaimLeaseTable(schedulesStream.Id), ds.Schema, prefix),
		fmt.Sprintf(`DELETE FROM %s.consumer_group_config WHERE name LIKE '%s.%%';`, ds.Schema, prefix),
	} {
		exec(ctx, sql)
	}
}

// --- capture logger ---

// captureLogger records every line so sections can count edges by their
// alert/owner attributes.
type captureLogger struct {
	mu    sync.Mutex
	lines []capturedLine
}

type capturedLine struct {
	level   string
	message string
	args    map[string]any
}

func newCaptureLogger() *captureLogger {
	return &captureLogger{}
}

func (c *captureLogger) record(level string, message string, args []any) {
	fields := map[string]any{}
	for i := 0; i+1 < len(args); i += 2 {
		if key, ok := args[i].(string); ok {
			fields[key] = args[i+1]
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, capturedLine{level: level, message: message, args: fields})
}

func (c *captureLogger) DebugContext(ctx context.Context, message string, args ...any) {
	c.record("debug", message, args)
}

func (c *captureLogger) InfoContext(ctx context.Context, message string, args ...any) {
	c.record("info", message, args)
}

func (c *captureLogger) WarnContext(ctx context.Context, message string, args ...any) {
	c.record("warn", message, args)
}

func (c *captureLogger) ErrorContext(ctx context.Context, message string, args ...any) {
	c.record("error", message, args)
}

// count is the number of lines at level carrying alert=alertName owner=ownerName.
func (c *captureLogger) count(level string, alertName string, ownerName string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	matches := 0
	for _, line := range c.lines {
		if line.level == level && line.args["alert"] == alertName && line.args["owner"] == ownerName {
			matches++
		}
	}
	return matches
}

// --- assertion helpers ---

// readCheckSummary returns the partition_count check summary heads by metric
// name -- the latest run's counts, read the same way `sqlstreams metric list`
// reads them.
func readCheckSummary(ctx context.Context) map[string]float64 {
	measurements, err := client.System().Metrics().Latest(ctx)
	common.Must(err)
	attributes := map[string]string{"alert": alert.AlertPartitionCount.Name}
	byKey := make(map[string]float64, len(measurements))
	for _, measurement := range measurements {
		byKey[iMetrics.MeasurementKey(measurement.Name, measurement.Attributes)] = measurement.Value
	}
	summary := make(map[string]float64, 4)
	for _, name := range []string{
		iMetrics.MetricCheckStreamsEvaluated.Name,
		iMetrics.MetricCheckStreamsFailed.Name,
		iMetrics.MetricCheckPublishedAlerts.Name,
		iMetrics.MetricCheckResolvedAlerts.Name,
	} {
		value, ok := byKey[iMetrics.MeasurementKey(name, attributes)]
		if !ok {
			common.Die(fmt.Sprintf("no check summary head for %s", name))
		}
		summary[name] = value
	}
	return summary
}

func partitionCountKey(owner *iCommon.Owner) string {
	key, err := alert.MessageKey(alert.AlertPartitionCount.Name, owner)
	common.Must(err)
	return key
}

func alertMessageCount(ctx context.Context, messageKey string) int64 {
	return scalarInt64(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE message_key = $1;`, ds.Schema, stream.MessageLogTable(alertsStream.Id)), messageKey)
}

func headId(ctx context.Context, messageKey string) int64 {
	return scalarInt64(ctx, fmt.Sprintf(`SELECT message_id FROM %s.%s WHERE compaction_key = $1;`, ds.Schema, stream.CompactionHeadTable(alertsStream.Id)),
		messageKey)
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

// waitDelivered returns once the partition_count group's delivery log holds
// the request at the given status.
func waitDelivered(ctx context.Context, messageId int64, status string) {
	waitForCount(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = '%s';`, ds.Schema, stream.DeliveryLogTable(schedulesStream.Id), partitionCountGroup, messageId, status), 1)
}

func waitForCount(ctx context.Context, sql string, want int64) {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if scalarInt64(ctx, sql) >= want {
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

func scalarString(ctx context.Context, sql string, args ...any) string {
	var value string
	common.Must(ds.Pool.QueryRow(ctx, sql, args...).Scan(&value))
	return value
}

func exec(ctx context.Context, sql string, args ...any) {
	_, err := ds.Pool.Exec(ctx, sql, args...)
	common.Must(err)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
