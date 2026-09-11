// Command schedule proves the schedule machinery end to end against a live
// scheduler worker claimed with a fast poll rate.
//
// Sections:
//  1. validation -- charset/star names, sub-minute and no-upcoming schedules,
//     timeout vs min rate, re-register wins, Feb-29 single-scheduled-time pass
//  2. target -- a schedule dies with its target stream; one targeting another
//     stream survives
//     2b. handle -- scheduler.Register declares the same row admin does, refuses
//     an unregistered stream and a bad expression, and Schedule runs the
//     system manager until its ctx cancels
//  3. produce-once -- a backdated row produces ONE message stamped with the
//     NEWEST due scheduled time, older dues dropped
//  4. v7 dedupe -- re-backdating to the SAME scheduled time is a Duplicate,
//     not a second message
//  5. suspend/unsuspend -- a suspended row never produces; a scheduled time
//     that came due while suspended is dropped, not produced late
//  6. poisoned row -- one job's produce fails every tick, siblings still
//     produce and the worker keeps ticking
//  7. exclusive (spot -- exclusive owns depth) -- a scheduler request lands while
//     a previous one is still running, waits, then runs
//  8. run-now beside a running request -- the default 'parallel' runs alongside
//     it; cfg.Concurrency exclusive waits for it instead
//  9. run-now supersedes a pending unclaimed request
//  10. consumer end-to-end + status -- bind the job's name, fail-once retry,
//     'success' rows land, ScheduleStatus shows RAN/SUCCEEDED/FAILED
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iCommon "github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	scheduleproducer "github.com/agentstax/sqlstreams/pkg/schedule/producer"
	"github.com/agentstax/sqlstreams/pkg/scheduler"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
)

const schedulerPollRate = 100 * time.Millisecond

var (
	ds            *iDatastore.PostgresDatastore
	client        *sqlstreams.Client
	testScheduler *scheduler.Scheduler
	target        *stream.Stream // the e2e test's own target stream
	prefix        string
)

// testMessage is what every e2e test schedule produces.
type testMessage struct {
	Kind string `json:"kind"`
}

func (testMessage) SchemaVersion() int { return 1 }

var payload = &testMessage{Kind: "e2e"}

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
	testScheduler, err = scheduler.NewScheduler(ds)
	common.Must(err)

	prefix = fmt.Sprintf("schedule.%d", time.Now().UnixNano())
	// status reads count 'success' rows, so the target keeps every outcome
	target, err = client.Stream[sqlstreams.RawPayload](prefix+".target").Register(ctx, &sqlstreams.StreamConfig{DeliveryLogMode: stream.DeliveryLogModeAll})
	common.Must(err)
	defer cleanupTarget()

	validationSection(ctx)
	targetSection(ctx)
	handleSection(ctx)

	// every scheduler-driven section below rides this one claimed instance
	stopScheduler := startScheduler(ctx)
	defer stopScheduler()

	produceOnceSection(ctx)
	dedupeSection(ctx)
	suspendSection(ctx)
	poisonSection(ctx)
	deferSection(ctx)
	runNowOverrideSection(ctx)
	supersedeSection(ctx)
	statusSection(ctx)

	fmt.Println("\n✅ SCHEDULE E2E TEST PASSED")
	return nil
}

// --- sections ---

func validationSection(ctx context.Context) {
	step("validation: names, schedules, timeout vs min rate, re-register wins")

	if _, err := registerSchedule(ctx, "", "@hourly", target.Name, payload, nil); err == nil {
		common.Die("empty name must be rejected")
	}
	if _, err := registerSchedule(ctx, prefix+".Upper", "@hourly", target.Name, payload, nil); err == nil {
		common.Die("uppercase name must be rejected")
	}
	if _, err := registerSchedule(ctx, prefix+".star*", "@hourly", target.Name, payload, nil); err == nil {
		common.Die("'*' in a name is the binding wildcard and must be rejected")
	}
	if _, err := schedule.ParseExpression("@every 30s"); err == nil {
		common.Die("sub-minute expression must be rejected at parse")
	}
	// Feb 30 never exists, so the schedule has no upcoming scheduled time
	if _, err := schedule.ParseExpression("0 0 30 2 *"); err == nil {
		common.Die("a expression with no upcoming scheduled time must be rejected at parse")
	}
	if _, err := registerSchedule(ctx, prefix+".validate", "@hourly", target.Name, payload, &scheduler.SchedulerConfig{Timeout: 2 * time.Hour}); err == nil {
		common.Die("timeout above the expression's min rate must be rejected")
	}
	fmt.Println("  ✓ rejections: empty/uppercase/star name, sub-minute, no-upcoming, timeout > min rate")

	// Feb-29 has under two scheduled times inside the min-rate horizon -- the
	// single-scheduled-time pass must register it, seeded on a real Feb 29
	feb29, err := registerSchedule(ctx, prefix+".feb29", "0 0 29 2 *", target.Name, payload, nil)
	common.Must(err)
	if feb29.NextScheduledAt.UTC().Month() != time.February || feb29.NextScheduledAt.UTC().Day() != 29 {
		common.Die(fmt.Sprintf("feb29 job seeded to %v, want a Feb 29", feb29.NextScheduledAt))
	}
	common.Must(client.Scheduler(prefix + ".feb29").Destroy(ctx))
	fmt.Printf("  ✓ Feb-29 expression registered, seeded to %s\n", feb29.NextScheduledAt.UTC().Format("2006-01-02"))

	first, err := registerSchedule(ctx, prefix+".redeclare", "@hourly", target.Name, payload, nil)
	common.Must(err)
	again, err := registerSchedule(ctx, prefix+".redeclare", "@hourly", target.Name, payload, nil)
	common.Must(err)
	if again.Id != first.Id {
		common.Die(fmt.Sprintf("identical re-register resolved to a different job: %d vs %d", again.Id, first.Id))
	}

	common.Must(client.Scheduler(prefix + ".redeclare").Suspend(ctx))
	redeclared, err := registerSchedule(ctx, prefix+".redeclare", "@daily", target.Name, payload, nil)
	common.Must(err)
	if redeclared.Id != first.Id {
		common.Die(fmt.Sprintf("re-register resolved to a different job: %d vs %d", redeclared.Id, first.Id))
	}
	if redeclared.Expression != "@daily" {
		common.Die(fmt.Sprintf("re-registered expression = %q, want %q", redeclared.Expression, "@daily"))
	}
	if !redeclared.NextScheduledAt.After(time.Now().UTC()) {
		common.Die(fmt.Sprintf("a expression change must re-seed the next scheduled time, got %v", redeclared.NextScheduledAt))
	}
	if !redeclared.Suspended {
		common.Die("a re-register must leave a suspended job suspended")
	}
	common.Must(client.Scheduler(prefix + ".redeclare").Destroy(ctx))
	fmt.Println("  ✓ identical re-register is a no-op, a differing one wins and leaves suspended alone")
}

func targetSection(ctx context.Context) {
	step("target: a schedule dies with its target stream, one on another stream survives")

	streamName := prefix + ".ownedstream"
	_, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, nil)
	common.Must(err)

	_, err = registerSchedule(ctx, prefix+".cascade", "@hourly", streamName, payload, nil)
	common.Must(err)
	_, err = registerSchedule(ctx, prefix+".standalone", "@hourly", target.Name, payload, nil)
	common.Must(err)

	common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))

	cascaded, err := client.Scheduler(prefix + ".cascade").Get(ctx)
	common.Must(err)
	if cascaded != nil {
		common.Die("a schedule must cascade away with its target stream")
	}
	standalone, err := client.Scheduler(prefix + ".standalone").Get(ctx)
	common.Must(err)
	if standalone == nil {
		common.Die("a schedule on another stream must survive an unrelated stream destroy")
	}
	common.Must(client.Scheduler(prefix + ".standalone").Destroy(ctx))
	fmt.Println("  ✓ cascade removed the schedule with its target stream, the other survived")
}

func handleSection(ctx context.Context) {
	step("handle: scheduler.Register declares the row, Schedule runs the manager until ctx cancels")

	if _, err := testScheduler.Register[testMessage](ctx, prefix+".handle", prefix+".missing", "@hourly", payload, nil); !errors.Is(err, stream.ErrStreamNotFound) {
		common.Die(fmt.Sprintf("want ErrStreamNotFound for an unregistered target, got %v", err))
	}
	if _, err := testScheduler.Register[testMessage](ctx, prefix+".handle", target.Name, "every day at noon", payload, nil); err == nil {
		common.Die("want an error for an unparseable expression")
	}

	nightly, err := testScheduler.Register[testMessage](ctx, prefix+".handle", target.Name, "@hourly", payload, nil)
	common.Must(err)
	defer func() { common.Must(client.Scheduler(prefix + ".handle").Destroy(ctx)) }()
	found, err := client.Scheduler(prefix + ".handle").Get(ctx)
	common.Must(err)
	if found == nil || found.Id != nightly.Registered.Id || found.StreamId != target.Id || found.SchemaVersion != 1 {
		common.Die(fmt.Sprintf("handle row differs from admin's read: %+v vs %+v", nightly.Registered, found))
	}
	if nightly.Payload != payload {
		common.Die("instance must keep the payload it registered")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	time.AfterFunc(2*time.Second, cancel)
	if err := nightly.Schedule(runCtx); err != nil {
		common.Die(fmt.Sprintf("Schedule must return nil on a requested stop, got %v", err))
	}
	fmt.Println("  ✓ handle registered the same row admin reads; Schedule ran the manager and stopped clean")
}

func produceOnceSection(ctx context.Context) {
	step("produce-once: a 5m-backdated row produces ONE message, stamped with the NEWEST due scheduled time")

	job, err := registerSchedule(ctx, prefix+".walk", "@every 1m", target.Name, payload, nil)
	common.Must(err)
	defer func() { common.Must(client.Scheduler(prefix + ".walk").Destroy(ctx)) }()
	key := job.Name

	backdated := time.Now().UTC().Add(-5 * time.Minute)
	backdate(ctx, job.Id, backdated)
	waitAdvanced(ctx, job.Id)

	if got := messageCount(ctx, key); got != 1 {
		common.Die(fmt.Sprintf("want exactly 1 message for the backdated row, got %d", got))
	}
	produced := producedScheduledTimes(ctx, key)[0]
	if produced.Sub(backdated) < 3*time.Minute {
		common.Die(fmt.Sprintf("produced scheduled time %v is too close to the backdate %v -- older dues were not dropped", produced, backdated))
	}
	if time.Since(produced) > 90*time.Second {
		common.Die(fmt.Sprintf("produced scheduled time %v is not the newest due", produced))
	}
	fmt.Printf("  ✓ 1 message, scheduled time %v after the backdate (newest due)\n", produced.Sub(backdated).Round(time.Second))
}

func dedupeSection(ctx context.Context) {
	step("v7 dedupe: re-backdating to the SAME scheduled time is a Duplicate, not a second message")

	job, err := registerSchedule(ctx, prefix+".dedupe", "@every 1m", target.Name, payload, nil)
	common.Must(err)
	defer func() { common.Must(client.Scheduler(prefix + ".dedupe").Destroy(ctx)) }()
	key := job.Name

	// 10s back: due now, and its successor stays out of reach for ~50s more,
	// so the re-backdated tick can only re-produce this exact scheduled time
	scheduledTime := time.Now().UTC().Add(-10 * time.Second)
	backdate(ctx, job.Id, scheduledTime)
	waitAdvanced(ctx, job.Id)
	if got := messageCount(ctx, key); got != 1 {
		common.Die(fmt.Sprintf("setup: want 1 message, got %d", got))
	}

	backdate(ctx, job.Id, scheduledTime)
	waitAdvanced(ctx, job.Id)
	if got := messageCount(ctx, key); got != 1 {
		common.Die(fmt.Sprintf("the same scheduled time produced twice: %d messages", got))
	}
	fmt.Println("  ✓ second tick on the same scheduled time deduped -- still 1 message")
}

func suspendSection(ctx context.Context) {
	step("suspend/unsuspend: a due-while-suspended scheduled time is dropped, not produced late")

	job, err := registerSchedule(ctx, prefix+".suspend", "@hourly", target.Name, payload, nil)
	common.Must(err)
	defer func() { common.Must(client.Scheduler(prefix + ".suspend").Destroy(ctx)) }()
	key := job.Name

	common.Must(client.Scheduler(prefix + ".suspend").Suspend(ctx))
	backdate(ctx, job.Id, time.Now().UTC().Add(-2*time.Hour))
	time.Sleep(10 * schedulerPollRate)
	if got := messageCount(ctx, key); got != 0 {
		common.Die(fmt.Sprintf("suspended row produced %d messages", got))
	}

	common.Must(client.Scheduler(prefix + ".suspend").Unsuspend(ctx))
	unsuspended, err := client.Scheduler(prefix + ".suspend").Get(ctx)
	common.Must(err)
	if !unsuspended.NextScheduledAt.After(time.Now()) {
		common.Die(fmt.Sprintf("unsuspend must re-seed next_scheduled_at in the future, got %v", unsuspended.NextScheduledAt))
	}
	time.Sleep(5 * schedulerPollRate)
	if got := messageCount(ctx, key); got != 0 {
		common.Die("the scheduled time that came due while suspended was produced late")
	}

	// positive control: the same row produces once genuinely due again
	backdate(ctx, job.Id, time.Now().UTC().Add(-2*time.Hour))
	waitAdvanced(ctx, job.Id)
	if got := messageCount(ctx, key); got != 1 {
		common.Die(fmt.Sprintf("unsuspended row should produce when due, got %d messages", got))
	}
	fmt.Println("  ✓ suspended row silent, unsuspend re-seeded forward, due row produced again")
}

func poisonSection(ctx context.Context) {
	step("poisoned row: one job's produce fails every tick, siblings still produce")

	poisoned, err := registerSchedule(ctx, prefix+".poison", "@hourly", target.Name, payload, nil)
	common.Must(err)
	defer func() { common.Must(client.Scheduler(prefix + ".poison").Destroy(ctx)) }()
	sibling, err := registerSchedule(ctx, prefix+".sibling", "@every 1m", target.Name, payload, nil)
	common.Must(err)
	defer func() { common.Must(client.Scheduler(prefix + ".sibling").Destroy(ctx)) }()
	poisonedKey := poisoned.Name
	siblingKey := sibling.Name

	// registration validated the schedule, so corrupt the row directly --
	// every ClaimDueSchedule's ParseSchedule now fails for this row
	exec(ctx, fmt.Sprintf(`UPDATE %s.schedule_config SET expression = 'not a expression' WHERE id = $1;`, ds.Schema), poisoned.Id)
	backdate(ctx, poisoned.Id, time.Now().UTC().Add(-2*time.Hour))

	backdate(ctx, sibling.Id, time.Now().UTC().Add(-10*time.Second))
	waitAdvanced(ctx, sibling.Id)
	if got := messageCount(ctx, siblingKey); got != 1 {
		common.Die(fmt.Sprintf("sibling should produce beside the poisoned row, got %d messages", got))
	}

	// the worker is still ticking: the sibling produces again while the
	// poisoned row keeps failing
	backdate(ctx, sibling.Id, time.Now().UTC().Add(-9*time.Second))
	waitAdvanced(ctx, sibling.Id)
	if got := messageCount(ctx, siblingKey); got != 2 {
		common.Die(fmt.Sprintf("worker should keep ticking past the poisoned row, got %d sibling messages", got))
	}
	if got := messageCount(ctx, poisonedKey); got != 0 {
		common.Die(fmt.Sprintf("poisoned row must not produce, got %d messages", got))
	}
	fmt.Println("  ✓ poisoned row produced nothing across 2 ticks, sibling produced both times")
}

func deferSection(ctx context.Context) {
	step("exclusive (spot): a scheduler request waits behind a running one, then runs")

	job, err := registerSchedule(ctx, prefix+".defer", "@every 1m", target.Name, payload,
		&scheduler.SchedulerConfig{Concurrency: iCommon.ConcurrencyExclusive})
	common.Must(err)
	defer func() { common.Must(client.Scheduler(prefix + ".defer").Destroy(ctx)) }()

	groupName := prefix + ".defer.group"
	group := registerGroup(ctx, groupName, prefix+".defer")

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	stop := startConsumer(ctx, groupName, []string{prefix + ".defer"}, 3, func(ctx context.Context, _ *testMessage) error {
		var first bool
		once.Do(func() { first = true })
		if first {
			close(started)
			<-release
		}
		return nil
	})
	defer stop()

	// both requests are scheduler-produced, so both carry the job's own
	// 'exclusive' -- the first is still running when the second lands (an 'parallel'
	// run never makes later requests wait, so run-now can't be the blocker)
	backdate(ctx, job.Id, time.Now().UTC().Add(-10*time.Second))
	waitAdvanced(ctx, job.Id)
	<-started

	backdate(ctx, job.Id, time.Now().UTC().Add(-9*time.Second))
	waitAdvanced(ctx, job.Id)
	deferred := scalarInt64(ctx, fmt.Sprintf(`SELECT MAX(id) FROM %s.%s WHERE message_key = $1;`, ds.Schema, stream.MessageLogTable(target.Id)),
		job.Name)

	// the 'deferred' row lands while the first request is still running
	waitForCount(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = 'deferred';`, ds.Schema, stream.DeliveryLogTable(target.Id), group, deferred), 1)
	close(release)
	waitForCount(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = 'success';`, ds.Schema, stream.DeliveryLogTable(target.Id), group, deferred), 1)
	fmt.Println("  ✓ scheduler request deferred behind the running one, then ran to success")
}

func runNowOverrideSection(ctx context.Context) {
	step("run-now beside a running request: default 'parallel' runs alongside it, cfg exclusive waits for it")

	job, err := registerSchedule(ctx, prefix+".runnow", "@hourly", target.Name, payload,
		&scheduler.SchedulerConfig{Concurrency: iCommon.ConcurrencyExclusive})
	common.Must(err)
	defer func() { common.Must(client.Scheduler(prefix + ".runnow").Destroy(ctx)) }()

	groupName := prefix + ".runnow.group"
	group := registerGroup(ctx, groupName, prefix+".runnow")

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	stop := startConsumer(ctx, groupName, []string{prefix + ".runnow"}, 3, func(ctx context.Context, _ *testMessage) error {
		var first bool
		once.Do(func() { first = true })
		if first {
			close(started)
			<-release
		}
		return nil
	})
	defer stop()

	// the blocker must be scheduler-produced: it carries the job's 'exclusive',
	// so later exclusive requests wait for it while the handler blocks
	backdate(ctx, job.Id, time.Now().UTC().Add(-2*time.Hour))
	waitAdvanced(ctx, job.Id)
	<-started
	blocker := scalarInt64(ctx, fmt.Sprintf(`SELECT MAX(id) FROM %s.%s WHERE message_key = $1;`, ds.Schema, stream.MessageLogTable(target.Id)),
		job.Name)

	// were the second request stamped with the job's 'exclusive', it would wait
	// until the first finishes -- the default 'parallel' runs it now
	override, err := client.Scheduler(prefix+".runnow").Run(ctx, nil)
	common.Must(err)
	waitForCount(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = 'success';`, ds.Schema, stream.DeliveryLogTable(target.Id), group, override.Id), 1)
	if got := scalarInt64(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = 'success';`, ds.Schema, stream.DeliveryLogTable(target.Id), group, blocker)); got != 0 {
		common.Die("the first request finished before the override ran -- the mid-run window was missed")
	}
	fmt.Println("  ✓ default run-now succeeded while the first was still running")

	// cfg.Concurrency exclusive opts back into the job's no-overlap safety: this
	// request waits for the running one instead of running beside it
	deferred, err := client.Scheduler(prefix+".runnow").Run(ctx, &sqlstreams.ScheduleRunOptions{Concurrency: iCommon.ConcurrencyExclusive})
	common.Must(err)
	waitForCount(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = 'deferred';`, ds.Schema, stream.DeliveryLogTable(target.Id), group, deferred.Id), 1)
	if got := scalarInt64(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = 'success';`, ds.Schema, stream.DeliveryLogTable(target.Id), group, deferred.Id)); got != 0 {
		common.Die("an exclusive run-now must not run while a previous request is still running")
	}
	close(release)
	waitForCount(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND status = 'success';`, ds.Schema, stream.DeliveryLogTable(target.Id), group), 3)
	fmt.Println("  ✓ exclusive run-now waited for the running request, then ran")
}

func supersedeSection(ctx context.Context) {
	step("run-now supersedes a pending unclaimed request")

	job, err := registerSchedule(ctx, prefix+".supersede", "@hourly", target.Name, payload, nil)
	common.Must(err)
	defer func() { common.Must(client.Scheduler(prefix + ".supersede").Destroy(ctx)) }()

	groupName := prefix + ".supersede.group"
	group := registerGroup(ctx, groupName, prefix+".supersede")

	// both requests land before any consumer claims -- the second takes over
	// the key's compaction_head pointer, and the claim query only returns a
	// keyed row while it IS the head, so the first is dropped unrun with no
	// delivery_log trace (the 'superseded' log row is the dispatched-then-
	// outraced exclusive path -- exclusive owns it)
	pending, err := client.Scheduler(prefix+".supersede").Run(ctx, nil)
	common.Must(err)
	head, err := client.Scheduler(prefix+".supersede").Run(ctx, nil)
	common.Must(err)

	if got := scalarInt64(ctx, fmt.Sprintf(`SELECT message_id FROM %s.%s WHERE compaction_key = $1;`, ds.Schema, stream.CompactionHeadTable(target.Id)),
		job.Name); got != head.Id {
		common.Die(fmt.Sprintf("the second run-now must take the compaction head, got %d want %d", got, head.Id))
	}

	var handled int64
	var mu sync.Mutex
	stop := startConsumer(ctx, groupName, []string{prefix + ".supersede"}, 1, func(ctx context.Context, _ *testMessage) error {
		mu.Lock()
		defer mu.Unlock()
		handled++
		return nil
	})
	defer stop()

	waitForCount(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = 'success';`, ds.Schema, stream.DeliveryLogTable(target.Id), group, head.Id), 1)
	if got := scalarInt64(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d;`, ds.Schema, stream.DeliveryLogTable(target.Id), group, pending.Id)); got != 0 {
		common.Die(fmt.Sprintf("the superseded request must leave no delivery rows, got %d", got))
	}
	mu.Lock()
	got := handled
	mu.Unlock()
	if got != 1 {
		common.Die(fmt.Sprintf("the superseded request must never reach the handler, handled %d", got))
	}

	statuses, err := client.Scheduler(prefix + ".supersede").Status(ctx)
	common.Must(err)
	status := statusFor(statuses, groupName)
	if status.Ran != 1 || status.Succeeded != 1 || status.Failed != 0 {
		common.Die(fmt.Sprintf("superseded requests must not count as ran: want 1/1/0, got %d/%d/%d", status.Ran, status.Succeeded, status.Failed))
	}
	if status.Superseded != 1 {
		common.Die(fmt.Sprintf("the dropped request must count as superseded: want 1, got %d", status.Superseded))
	}

	// the request listing names the replacement: newest first, the dropped
	// request points at the one that replaced it
	requests, err := client.Scheduler(prefix+".supersede").Messages(ctx, 20)
	common.Must(err)
	if len(requests) != 2 {
		common.Die(fmt.Sprintf("want 2 listed requests, got %d", len(requests)))
	}
	newest, oldest := requests[0], requests[1]
	if newest.MessageId != head.Id || newest.Outcome != schedule.ScheduleMessageSucceeded {
		common.Die(fmt.Sprintf("newest request: want %d succeeded, got %d %s", head.Id, newest.MessageId, newest.Outcome))
	}
	if oldest.MessageId != pending.Id || oldest.Outcome != schedule.ScheduleMessageSuperseded {
		common.Die(fmt.Sprintf("oldest request: want %d superseded, got %d %s", pending.Id, oldest.MessageId, oldest.Outcome))
	}
	if oldest.SupersededBy == nil || *oldest.SupersededBy != head.Id || oldest.SupersededAt == nil {
		common.Die(fmt.Sprintf("the dropped request must name its replacement %d, got %+v", head.Id, oldest.SupersededBy))
	}
	fmt.Println("  ✓ first request superseded unrun, only the second ran; status counts 1/1/0 with superseded=1")
	fmt.Printf("  ✓ request listing: %d succeeded; %d superseded by %d at %s\n",
		newest.MessageId, oldest.MessageId, *oldest.SupersededBy, oldest.SupersededAt.Format("15:04:05"))
}

func statusSection(ctx context.Context) {
	step("consumer end-to-end + status: fail-once retry, always-failing sibling, RAN/SUCCEEDED/FAILED")

	jobName := prefix + ".status"
	_, err := registerSchedule(ctx, jobName, "@hourly", target.Name, payload, nil)
	common.Must(err)
	defer func() { common.Must(client.Scheduler(jobName).Destroy(ctx)) }()

	boundName := prefix + ".status.bound"
	otherName := prefix + ".status.other"
	bindinglessName := prefix + ".status.bindingless"
	bound := registerGroup(ctx, boundName, jobName)
	registerGroup(ctx, otherName, "some.other.job")
	registerGroup(ctx, bindinglessName)

	// first request: fail once then succeed (the retry); later requests fail
	// every attempt
	var mu sync.Mutex
	attempts := map[time.Time]int{}
	var firstScheduledTime time.Time
	stop := startConsumer(ctx, boundName, []string{jobName}, 1, func(ctx context.Context, _ *testMessage) error {
		meta, _ := consume.MetaFromContext(ctx)
		mu.Lock()
		defer mu.Unlock()
		if firstScheduledTime.IsZero() {
			firstScheduledTime = meta.ScheduledAt
		}
		attempts[meta.ScheduledAt]++
		if meta.ScheduledAt.Equal(firstScheduledTime) && attempts[meta.ScheduledAt] > 1 {
			return nil
		}
		return errors.New("scripted failure")
	})
	defer stop()

	// wait out request 1's success before producing request 2, so the second
	// run-now can't supersede the first while it sits unclaimed
	first, err := client.Scheduler(jobName).Run(ctx, nil)
	common.Must(err)
	waitForCount(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = 'success';`, ds.Schema, stream.DeliveryLogTable(target.Id), bound, first.Id), 1)
	if got := scalarInt64(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = 'failure';`, ds.Schema, stream.DeliveryLogTable(target.Id), bound, first.Id)); got < 1 {
		common.Die("the first request must record its failed attempt before succeeding")
	}

	second, err := client.Scheduler(jobName).Run(ctx, nil)
	common.Must(err)
	waitForCount(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = %d AND message_id = %d AND status = 'failure';`, ds.Schema, stream.DeliveryLogTable(target.Id), bound, second.Id), 1)

	statuses, err := client.Scheduler(jobName).Status(ctx)
	common.Must(err)
	for _, status := range statuses {
		fmt.Printf("  group=%s ran=%d succeeded=%d failed=%d superseded=%d\n", status.ConsumerGroup, status.Ran, status.Succeeded, status.Failed, status.Superseded)
	}
	if len(statuses) != 2 {
		common.Die(fmt.Sprintf("want 2 matching groups (bound + bindingless), got %d", len(statuses)))
	}
	if statusFor(statuses, otherName) != nil {
		common.Die("a group bound to a different name must not match")
	}
	boundStatus := statusFor(statuses, boundName)
	if boundStatus == nil || boundStatus.Ran != 2 || boundStatus.Succeeded != 1 || boundStatus.Failed != 1 {
		common.Die(fmt.Sprintf("bound group: want ran=2 succeeded=1 failed=1, got %+v", boundStatus))
	}
	// the bound group RAN the first request before the second replaced it, so
	// nothing is superseded for it -- the bindingless group never ran it and
	// can never receive it now, so for that group it was dropped unrun
	if boundStatus.Superseded != 0 {
		common.Die(fmt.Sprintf("bound group ran every request: want superseded=0, got %d", boundStatus.Superseded))
	}
	bindingless := statusFor(statuses, bindinglessName)
	if bindingless == nil || bindingless.Ran != 0 || bindingless.Succeeded != 0 || bindingless.Failed != 0 {
		common.Die(fmt.Sprintf("bindingless group: want a 0/0/0 row, got %+v", bindingless))
	}
	if bindingless.Superseded != 1 {
		common.Die(fmt.Sprintf("bindingless group: the replaced first request was dropped unrun for it, want superseded=1, got %d", bindingless.Superseded))
	}
	fmt.Println("  ✓ retried-then-succeeded counts once; bound 2/1/1 superseded=0, bindingless 0/0/0 superseded=1, non-matching absent")

	// per-group outcomes for the same two requests: the bound group ran both,
	// the bindingless group ran neither
	requests, err := client.Scheduler(jobName).Messages(ctx, 20)
	common.Must(err)
	outcomes := map[string]schedule.ScheduleMessageOutcome{}
	for _, request := range requests {
		outcomes[fmt.Sprintf("%s/%d", request.ConsumerGroup, request.MessageId)] = request.Outcome
	}
	want := map[string]schedule.ScheduleMessageOutcome{
		fmt.Sprintf("%s/%d", boundName, first.Id):        schedule.ScheduleMessageSucceeded,
		fmt.Sprintf("%s/%d", boundName, second.Id):       schedule.ScheduleMessageFailed,
		fmt.Sprintf("%s/%d", bindinglessName, first.Id):  schedule.ScheduleMessageSuperseded,
		fmt.Sprintf("%s/%d", bindinglessName, second.Id): schedule.ScheduleMessagePending,
	}
	if len(requests) != len(want) {
		common.Die(fmt.Sprintf("want %d listed (request, group) rows, got %d", len(want), len(requests)))
	}
	for key, outcome := range want {
		if outcomes[key] != outcome {
			common.Die(fmt.Sprintf("request %s: want %s, got %s", key, outcome, outcomes[key]))
		}
	}
	fmt.Println("  ✓ request listing per group: bound succeeded+failed, bindingless superseded+pending")
}

// --- harness ---

// startScheduler claims the system's schedule producer worker row with the e2e test's
// fast poll rate and runs it until the returned stop is called.
func startScheduler(ctx context.Context) func() {
	sys, err := client.System().Get(ctx)
	common.Must(err)
	owner, err := iCommon.NewSystemOwner(sys.Id)
	common.Must(err)
	workers, err := workercontroller.NewWorkerController(ds, ds.Logger)
	common.Must(err)
	row, err := workers.GetWorker(ctx, scheduleproducer.WorkerScheduleProducer, owner)
	common.Must(err)

	provisioner, err := scheduleproducer.NewScheduleProducerProvisioner(ds, nil, ds.Logger)
	common.Must(err)

	// a crashed earlier run's claim lingers until its InstanceTTL expires --
	// retry past it instead of dying
	row.Metadata = map[string]any{"poll_rate": int64(schedulerPollRate)}

	var execution worker.Execution
	deadline := time.Now().Add(60 * time.Second)
	for {
		execution, err = provisioner.Provision(ctx, row)
		common.Must(err)
		if execution != nil {
			break
		}
		if time.Now().After(deadline) {
			common.Die("schedule producer declined the instance for 60s -- is another claimant running?")
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

// registerGroup creates the consumer group on the e2e test's target stream, bound
// to the given schedule names (none = bindingless), and returns its id.
func registerGroup(ctx context.Context, name string, bindings ...string) int64 {
	controller, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)
	group, err := controller.RegisterGroup(ctx, target.Id, name, consume.Beginning())
	common.Must(err)
	_, err = controller.DeclareBindings(ctx, target.Id, group.Id, bindings, time.Now())
	common.Must(err)
	return group.Id
}

// startConsumer runs one consumer instance on the group until the returned
// stop is called.
func startConsumer(ctx context.Context, group string, bindings []string, concurrency int, handler func(context.Context, *testMessage) error) func() {
	lifecycleCtx, cancel := context.WithCancel(ctx)
	instance, err := client.Stream[testMessage](target.Name).Consumer(group).Register(lifecycleCtx, &sqlstreams.ConsumerConfig{
		Bindings:                bindings,
		ExceptionInitialBackoff: 200 * time.Millisecond,
	})

	common.Must(err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = instance.Consume(lifecycleCtx, handler, &sqlstreams.ConsumeOptions{
			ClaimPollRate:      schedulerPollRate,
			MessageConcurrency: concurrency,
		})
	}()
	return func() {
		cancel()
		<-done
	}
}

func statusFor(statuses []*schedule.ScheduleConsumerGroupSummary, group string) *schedule.ScheduleConsumerGroupSummary {
	for _, status := range statuses {
		if status.ConsumerGroup == group {
			return status
		}
	}
	return nil
}

func cleanupTarget() {
	common.Must(client.Stream[testMessage](target.Name).Destroy(context.Background(), &sqlstreams.DestroyOptions{Force: true}))
}

// --- assertion helpers ---

func backdate(ctx context.Context, jobId int64, to time.Time) {
	exec(ctx, fmt.Sprintf(`UPDATE %s.schedule_cursor SET next_scheduled_at = $1 WHERE schedule_id = $2;`, ds.Schema), to, jobId)
}

// waitAdvanced returns once the scheduler has moved the row's
// next_scheduled_at back into the future -- its tick on the row is done.
func waitAdvanced(ctx context.Context, jobId int64) {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var advanced bool
		common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT next_scheduled_at > now() FROM %s.schedule_cursor WHERE schedule_id = $1;`, ds.Schema), jobId).Scan(&advanced))
		if advanced {
			return
		}
		time.Sleep(schedulerPollRate / 2)
	}
	common.Die(fmt.Sprintf("timed out waiting for schedule %d to advance", jobId))
}

func messageCount(ctx context.Context, messageKey string) int64 {
	return scalarInt64(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE message_key = $1;`, ds.Schema, stream.MessageLogTable(target.Id)), messageKey)
}

func producedScheduledTimes(ctx context.Context, messageKey string) []time.Time {
	rows, err := ds.Pool.Query(ctx, fmt.Sprintf(`SELECT options->>'scheduled_at' FROM %s.%s WHERE message_key = $1 ORDER BY id;`, ds.Schema, stream.MessageLogTable(target.Id)), messageKey)
	common.Must(err)
	defer rows.Close()

	var times []time.Time
	for rows.Next() {
		var raw string
		common.Must(rows.Scan(&raw))
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		common.Must(err)
		times = append(times, parsed)
	}
	common.Must(rows.Err())
	return times
}

func waitForCount(ctx context.Context, sql string, want int64) {
	deadline := time.Now().Add(20 * time.Second)
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

func exec(ctx context.Context, sql string, args ...any) {
	_, err := ds.Pool.Exec(ctx, sql, args...)
	common.Must(err)
}

// registerSchedule is the handle's Register for the e2e test's message type,
// returning the row like admin's reads do.
func registerSchedule(ctx context.Context, name string, expression string, streamName string, payload *testMessage, cfg *scheduler.SchedulerConfig) (*schedule.Schedule, error) {
	instance, err := testScheduler.Register[testMessage](ctx, name, streamName, expression, payload, cfg)
	if err != nil {
		return nil, err
	}
	return instance.Registered, nil
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
