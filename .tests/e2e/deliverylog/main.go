package main

// delivery_log e2e test: does the per-attempt audit trail actually behave like an
// audit trail -- one row per failed attempt, distinct rows (not overwrites)
// across retries -- and do the three delivery_log_mode settings and retention
// cleanup around it actually hold?
//
// Five scenarios, driven through the real consumer.Datastore methods (Commit,
// ClaimExceptions, RecordExceptionFailure, RecordExceptionSuccess,
// DropExpiredPartitions, SweepExpiredPartitions) rather than raw SQL:
//  1. under the default mode ('failures') a fresh failure logs exactly one
//     delivery_log row (attempt=0, the right error), a success in the same
//     Commit logs none.
//  2. retrying that same message twice logs two MORE distinct rows
//     (attempt=1, attempt=2) -- the PK is (consumer_group, message_id,
//     attempt), so a retry can never collide with or overwrite a prior one.
//  3. a stream registered with DeliveryLogModeOff silently skips every write
//     path (the table itself always exists, so re-enabling needs no DDL) --
//     a failure still writes its delivery row normally in delivery_<id>, just with no shadow
//     row.
//  4. a stream registered with DeliveryLogModeAll logs a 'success' row per
//     success, in the same txn as the success itself: Commit logs its
//     resolved successes, and an exception that later succeeds logs
//     'success' at its own attempt as its delivery row deletes.
//  5. retention (dropPartition's whole-partition removal, sweepBatch's
//     individually-expired-row reap) actually drains old delivery_log rows,
//     not just delivery_<id>'s.
//  6. a retry claim handed back at a busy key gate logs 'deferred' under the
//     number it returned, and the next claim's failure logs under that same
//     number beside it -- two events for one run, no key collision [0615].

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iCommon "github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	exceptionconsumercontroller "github.com/agentstax/sqlstreams/pkg/consume/exceptionconsumer/controller"
	messageconsumercontroller "github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	janitordatastore "github.com/agentstax/sqlstreams/pkg/stream/janitor/controller/datastore"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	group     = "phase11.deliverylog"
	ttl       = 100 * time.Millisecond
	ttlMargin = 300 * time.Millisecond
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

	scenarioFreshFailureAndSuccess(ctx, pool)
	scenarioRetryDistinctAttempts(ctx, pool)
	scenarioDeliveryLogOff(ctx, pool)
	scenarioDeliveryLogAll(ctx, pool)
	scenarioRetentionDropPartition(ctx, pool)
	scenarioRetentionSweepBatch(ctx, pool)
	scenarioRedeferralSharesAttempt(ctx, pool)

	fmt.Println("\n✅ DELIVERY LOG E2E TEST PASSED")
	fmt.Println("   a failure logs exactly one row, retries append distinct rows instead of")
	fmt.Println("   overwriting, mode 'off' skips every write entirely, mode 'all' logs a")
	fmt.Println("   'success' row per success in the success's own txn, and both retention")
	fmt.Println("   paths drain delivery_log the same as they already drain delivery_<id>.")
	return nil
}

// ---- scenario 1: fresh failure logs one row, success logs none ----

func scenarioFreshFailureAndSuccess(ctx context.Context, pool *pgxpool.Pool) {
	step("SCENARIO 1: a fresh failure logs one delivery_log row, a success logs none")

	tp, cd, wp, groupId := newStream(ctx, pool, "scenario1", sqlstreams.StreamConfig{})
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	defer func() {
		common.Must(client.Stream[common.Work](tp.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	seed(ctx, wp, 2)
	claim, err := cd.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, 2, 3, 5*time.Second, tp.DeliveryLogMode)
	common.Must(err)
	if claim == nil || len(claim.Messages) != 2 {
		common.Die("expected a fresh claim of 2 messages")
	}
	failingId, successId := claim.Messages[0].Id, claim.Messages[1].Id

	exceptions := []messageconsumercontroller.MessageOutcome{{MessageId: failingId, Kind: messageconsumercontroller.OutcomeException, Err: "simulated processing failure"}}
	common.Must(cd.Commit(ctx, tp.Id, groupId, claim.Lease.Token, exceptions, 300*time.Millisecond, tp.DeliveryLogMode))

	assertDeliveryLogRow(ctx, ds, tp.Id, groupId, failingId, 0, "simulated processing failure", true)
	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, successId, 0)
	fmt.Println("PASS: failure logged exactly one row, success logged none")
}

// ---- scenario 2: two retries append two more distinct rows ----

func scenarioRetryDistinctAttempts(ctx context.Context, pool *pgxpool.Pool) {
	step("SCENARIO 2: retrying the same message twice appends attempt=1 then attempt=2, never overwrites")

	tp, cd, wp, groupId := newStream(ctx, pool, "scenario2", sqlstreams.StreamConfig{})
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)
	exceptionConsumers, err := exceptionconsumercontroller.NewExceptionConsumerGroupController(ds, ds.Logger)
	common.Must(err)

	defer func() {
		common.Must(client.Stream[common.Work](tp.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	seed(ctx, wp, 1)
	claim, err := cd.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, 1, 3, 5*time.Second, tp.DeliveryLogMode)
	common.Must(err)
	if claim == nil {
		common.Die("expected a fresh claim")
	}
	failingId := claim.Messages[0].Id

	exceptions := []messageconsumercontroller.MessageOutcome{{MessageId: failingId, Kind: messageconsumercontroller.OutcomeException, Err: "attempt 0 failure"}}
	common.Must(cd.Commit(ctx, tp.Id, groupId, claim.Lease.Token, exceptions, 300*time.Millisecond, tp.DeliveryLogMode))
	assertDeliveryLogRow(ctx, ds, tp.Id, groupId, failingId, 0, "attempt 0 failure", true)

	const maxAttempts = 5 // stays well below dead-letter for both retries below
	for _, attempt := range []int{1, 2} {
		time.Sleep(1500 * time.Millisecond) // outlives both the 300ms initial and CalculateDelay(0)=1s can_run_after
		claimed, err := exceptionConsumers.Claim(ctx, tp.Id, groupId, 1, 10, maxAttempts, 5*time.Second, tp.DeliveryLogMode)
		common.Must(err)
		if len(claimed) != 1 || claimed[0].MessageId != failingId {
			common.Die(fmt.Sprintf("expected to claim exactly message %d, got %+v", failingId, claimed))
		}
		errText := fmt.Sprintf("attempt %d failure", attempt)
		common.Must(exceptionConsumers.RecordFailure(ctx, (&iCommon.RetryPolicy{MaxRetries: maxAttempts}).WithDefaults(), &claimed[0], fmt.Errorf("%s", errText), tp.DeliveryLogMode, nil))
		assertDeliveryLogRow(ctx, ds, tp.Id, groupId, failingId, attempt, errText, true)
	}

	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, failingId, 3) // attempt 0, 1, 2 -- three distinct rows
	fmt.Println("PASS: two retries appended two distinct rows, no overwrite of the original")
}

// ---- scenario 3: DeliveryLogModeOff skips every write ----

func scenarioDeliveryLogOff(ctx context.Context, pool *pgxpool.Pool) {
	step("SCENARIO 3: DeliveryLogModeOff skips every write (the table itself always exists)")

	tp, cd, wp, groupId := newStream(ctx, pool, "scenario3", sqlstreams.StreamConfig{DeliveryLogMode: stream.DeliveryLogModeOff})
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	defer func() {
		common.Must(client.Stream[common.Work](tp.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	// registration creates delivery_log_<id> regardless of the flag -- the
	// flag gates the writes, so re-enabling later needs no DDL
	assertTableExists(ctx, ds, fmt.Sprintf("%s.%s", ds.Schema, stream.DeliveryLogTable(tp.Id)), true)

	seed(ctx, wp, 1)
	claim, err := cd.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, 1, 3, 5*time.Second, tp.DeliveryLogMode)
	common.Must(err)
	if claim == nil {
		common.Die("expected a fresh claim")
	}
	failingId := claim.Messages[0].Id
	exceptions := []messageconsumercontroller.MessageOutcome{{MessageId: failingId, Kind: messageconsumercontroller.OutcomeException, Err: "should never be logged"}}
	common.Must(cd.Commit(ctx, tp.Id, groupId, claim.Lease.Token, exceptions, 300*time.Millisecond, tp.DeliveryLogMode))

	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, failingId, 0) // the failure was never logged
	assertDeliveryRowCount(ctx, ds, tp.Id, 1)                     // the delivery row was still written
	fmt.Println("PASS: no delivery_log row written, failure delivery row still written normally in delivery_<id>, no error")
}

// ---- scenario 4: DeliveryLogModeAll logs successes in the success's own txn ----

func scenarioDeliveryLogAll(ctx context.Context, pool *pgxpool.Pool) {
	step("SCENARIO 4: DeliveryLogModeAll logs a 'success' row per success, same txn as the success")

	tp, cd, wp, groupId := newStream(ctx, pool, "scenario4all", sqlstreams.StreamConfig{DeliveryLogMode: stream.DeliveryLogModeAll})
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)
	exceptionConsumers, err := exceptionconsumercontroller.NewExceptionConsumerGroupController(ds, ds.Logger)
	common.Must(err)

	defer func() {
		common.Must(client.Stream[common.Work](tp.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	seed(ctx, wp, 2)
	claim, err := cd.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, 2, 3, 5*time.Second, tp.DeliveryLogMode)
	common.Must(err)
	if claim == nil || len(claim.Messages) != 2 {
		common.Die("expected a fresh claim of 2 messages")
	}
	failingId, successId := claim.Messages[0].Id, claim.Messages[1].Id

	// one failure and one success in the same Commit -- the success rides the
	// outcome list as OutcomeSuccess, the shape the consumer runner uses
	// under this mode
	outcomes := []messageconsumercontroller.MessageOutcome{
		{MessageId: failingId, Kind: messageconsumercontroller.OutcomeException, Err: "scenario 4 failure"},
		{MessageId: successId, Kind: messageconsumercontroller.OutcomeSuccess},
	}
	common.Must(cd.Commit(ctx, tp.Id, groupId, claim.Lease.Token, outcomes, 300*time.Millisecond, tp.DeliveryLogMode))

	assertDeliveryLogStatus(ctx, ds, tp.Id, groupId, successId, 0, "success")
	assertDeliveryLogStatus(ctx, ds, tp.Id, groupId, failingId, 0, "failure")

	// the unresolved exception now succeeds on its retry -- the delivery row's
	// deletion and its 'success' log row are one statement
	time.Sleep(1500 * time.Millisecond) // outlives the 300ms initial can_run_after
	claimed, err := exceptionConsumers.Claim(ctx, tp.Id, groupId, 1, 10, 5, 5*time.Second, tp.DeliveryLogMode)
	common.Must(err)
	if len(claimed) != 1 || claimed[0].MessageId != failingId {
		common.Die(fmt.Sprintf("expected to claim exactly message %d, got %+v", failingId, claimed))
	}
	common.Must(exceptionConsumers.RecordSuccess(ctx, &claimed[0], tp.DeliveryLogMode, nil))

	assertDeliveryLogStatus(ctx, ds, tp.Id, groupId, failingId, claimed[0].Attempts, "success")
	assertDeliveryRowCount(ctx, ds, tp.Id, 0) // the success-deletion still happened
	fmt.Println("PASS: commit logged the success, the exception's later success logged at its own attempt")
}

// ---- scenario 5: retention drains old delivery_log rows ----

func scenarioRetentionDropPartition(ctx context.Context, pool *pgxpool.Pool) {
	step("SCENARIO 5a: dropPartition reaps a dormant message's delivery_log row")

	const partitionSize = int64(4)
	tp, cd, wp, groupId := newStream(ctx, pool, "scenario4drop", sqlstreams.StreamConfig{PartitionSize: partitionSize})
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)
	janitorDatastore, err := janitordatastore.NewJanitorDatastore(ds, ds.Logger)
	common.Must(err)

	defer func() {
		common.Must(client.Stream[common.Work](tp.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	dormantId := failOne(ctx, cd, wp, tp, groupId, 4) // fills partition 0 (ids 1-4), fails id 1
	time.Sleep(ttl + ttlMargin)
	aliveId := failOne(ctx, cd, wp, tp, groupId, 4) // rolls into partition 1 (ids 5-8), fails id 5 -- well inside ttl

	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, dormantId, 1)
	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, aliveId, 1)

	common.Must(janitorDatastore.DropExpiredPartitions(ctx, tp.Id, partitionSize, ttl, true, tp.DeliveryLogMode))

	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, dormantId, 0)
	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, aliveId, 1)
	fmt.Println("PASS: dropPartition reaped the dormant message's delivery_log row, left the alive one")
}

func scenarioRetentionSweepBatch(ctx context.Context, pool *pgxpool.Pool) {
	step("SCENARIO 5b: sweepBatch reaps a dormant message's delivery_log row individually")

	const partitionSize = int64(1000000) // never rolls -- exercises the sweep path instead of the drop
	tp, cd, wp, groupId := newStream(ctx, pool, "scenario4sweep", sqlstreams.StreamConfig{PartitionSize: partitionSize})
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)
	janitorDatastore, err := janitordatastore.NewJanitorDatastore(ds, ds.Logger)
	common.Must(err)

	defer func() {
		common.Must(client.Stream[common.Work](tp.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	dormantId := failOne(ctx, cd, wp, tp, groupId, 1)
	time.Sleep(ttl + ttlMargin)
	aliveId := failOne(ctx, cd, wp, tp, groupId, 1) // well inside ttl

	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, dormantId, 1)
	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, aliveId, 1)

	common.Must(janitorDatastore.SweepExpiredPartitions(ctx, tp.Id, partitionSize, ttl, 0, true, 1000, tp.DeliveryLogMode))

	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, dormantId, 0)
	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, aliveId, 1)
	fmt.Println("PASS: sweepBatch reaped the dormant message's delivery_log row, left the alive one")
}

// ---- scenario 6: a gate re-deferral and the run after it share an attempt ----

func scenarioRedeferralSharesAttempt(ctx context.Context, pool *pgxpool.Pool) {
	step("SCENARIO 6: a claim handed back at the key gate and the next run log under the same attempt")

	tp, _, _, groupId := newStream(ctx, pool, "scenario6", sqlstreams.StreamConfig{})
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)
	exceptionConsumers, err := exceptionconsumercontroller.NewExceptionConsumerGroupController(ds, ds.Logger)
	common.Must(err)

	defer func() {
		common.Must(client.Stream[common.Work](tp.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	// a keyed message with its first-delivery 'deferred' row, as the cursor path writes it
	var messageId int64
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.%s (message_key, schema_version, payload) VALUES ('k', 1, '{}') RETURNING id`, ds.Schema, stream.MessageLogTable(tp.Id))).Scan(&messageId))
	_, err = ds.Pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.%s (consumer_group_id, message_id, status, concurrency, attempts) VALUES ($1, $2, 'deferred', 'exclusive', 0)`, ds.Schema, stream.ExceptionQueueTable(tp.Id)), groupId, messageId)
	common.Must(err)
	_, err = ds.Pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.%s (consumer_group_id, message_id, attempt, status, error) VALUES ($1, $2, 0, 'deferred', '')`, ds.Schema, stream.DeliveryLogTable(tp.Id)), groupId, messageId)
	common.Must(err)

	claimed, err := exceptionConsumers.Claim(ctx, tp.Id, groupId, 1, 10, 3, 5*time.Second, tp.DeliveryLogMode)
	common.Must(err)
	if len(claimed) != 1 || claimed[0].Attempts != 1 {
		common.Die(fmt.Sprintf("expected one claim at attempts 1, got %+v", claimed))
	}
	common.Must(exceptionConsumers.RecordDeferred(ctx, &claimed[0], iCommon.ConcurrencyExclusive, tp.DeliveryLogMode))

	claimed, err = exceptionConsumers.Claim(ctx, tp.Id, groupId, 1, 10, 3, 5*time.Second, tp.DeliveryLogMode)
	common.Must(err)
	if len(claimed) != 1 || claimed[0].Attempts != 1 {
		common.Die(fmt.Sprintf("expected the handed-back number 1 to be claimed again, got %+v", claimed))
	}
	common.Must(exceptionConsumers.RecordFailure(ctx, (&iCommon.RetryPolicy{MaxRetries: 3}).WithDefaults(), &claimed[0], fmt.Errorf("attempt 1 failure"), tp.DeliveryLogMode, nil))

	assertDeliveryLogStatusesAt(ctx, ds, tp.Id, groupId, messageId, 1, []string{"deferred", "failure"})
	assertDeliveryLogCount(ctx, ds, tp.Id, groupId, messageId, 3)
	fmt.Println("PASS: the gate deferral and the run after it both logged under attempt 1")
}

// ---- helpers ----

func newStream(ctx context.Context, pool *pgxpool.Pool, suffix string, cfg sqlstreams.StreamConfig) (*stream.Stream, *messageconsumercontroller.MessageConsumerGroupController, *sqlstreams.ProducerInstance[common.Work], int64) {
	name := fmt.Sprintf("phase11.deliverylog.%s.%d", suffix, time.Now().UnixNano())
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)

	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)
	tp, err := client.Stream[sqlstreams.RawPayload](name).Register(ctx, &cfg)
	common.Must(err)

	cd, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)
	groupId := mustGroupID(cd.RegisterGroup(ctx, tp.Id, group, consume.Beginning()))
	messageConsumers, err := messageconsumercontroller.NewMessageConsumerGroupController(ds, ds.Logger)
	common.Must(err)
	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)
	return tp, messageConsumers, wpInstance, groupId
}

func seed(ctx context.Context, wpInstance *sqlstreams.ProducerInstance[common.Work], n int) {
	for range n {
		_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
			return common.NewWork(30, "admin@example.com")
		}, nil)
		common.Must(err)
	}
}

// failOne claims a fresh range of n messages and fails the first one -- returns
// its id. Used by the retention scenarios, which only care about one failure
// per range, not the retry-distinctness scenario 2 already covers.
func failOne(ctx context.Context, cd *messageconsumercontroller.MessageConsumerGroupController, wpInstance *sqlstreams.ProducerInstance[common.Work], tp *stream.Stream, groupId int64, n int) int64 {
	seed(ctx, wpInstance, n)
	claim, err := cd.ClaimMessagesWithCursor(ctx, tp.Id, groupId, 1, n, 3, 5*time.Second, tp.DeliveryLogMode)
	common.Must(err)
	if claim == nil {
		common.Die("expected a fresh claim")
	}
	failingId := claim.Messages[0].Id
	exceptions := []messageconsumercontroller.MessageOutcome{{MessageId: failingId, Kind: messageconsumercontroller.OutcomeException, Err: "retention scenario failure"}}
	common.Must(cd.Commit(ctx, tp.Id, groupId, claim.Lease.Token, exceptions, 300*time.Millisecond, tp.DeliveryLogMode))
	return failingId
}

func assertDeliveryLogRow(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, groupId int64, messageId int64, attempt int, wantErr string, wantExists bool) {
	var gotErr string
	err := ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT error FROM %s.%s WHERE consumer_group_id = $1 AND message_id = $2 AND attempt = $3;`, ds.Schema, stream.DeliveryLogTable(streamId)), groupId, messageId, attempt).Scan(&gotErr)
	exists := err == nil
	if exists != wantExists {
		common.Die(fmt.Sprintf("%s.%s[group=%d message=%d attempt=%d] exists=%v, want %v (err=%v)", ds.Schema, stream.DeliveryLogTable(streamId), groupId, messageId, attempt, exists, wantExists, err))
	}
	if wantExists && gotErr != wantErr {
		common.Die(fmt.Sprintf("%s.%s[message=%d attempt=%d] error=%q, want %q", ds.Schema, stream.DeliveryLogTable(streamId), messageId, attempt, gotErr, wantErr))
	}
	fmt.Printf("  ✓ delivery_log_%d[message=%d attempt=%d] exists=%v%s\n", streamId, messageId, attempt, exists, errSuffix(wantExists, gotErr))
}

func errSuffix(wantExists bool, gotErr string) string {
	if !wantExists {
		return ""
	}
	return fmt.Sprintf(" error=%q", gotErr)
}

func assertDeliveryLogStatus(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, groupId int64, messageId int64, attempt int, wantStatus string) {
	var gotStatus string
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.%s WHERE consumer_group_id = $1 AND message_id = $2 AND attempt = $3;`, ds.Schema, stream.DeliveryLogTable(streamId)), groupId, messageId, attempt).Scan(&gotStatus))
	if gotStatus != wantStatus {
		common.Die(fmt.Sprintf("%s.%s[message=%d attempt=%d] status=%q, want %q", ds.Schema, stream.DeliveryLogTable(streamId), messageId, attempt, gotStatus, wantStatus))
	}
	fmt.Printf("  ✓ delivery_log_%d[message=%d attempt=%d] status=%q\n", streamId, messageId, attempt, gotStatus)
}

// assertDeliveryLogStatusesAt checks every event logged under one attempt, in
// insertion order.
func assertDeliveryLogStatusesAt(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, groupId int64, messageId int64, attempt int, want []string) {
	rows, err := ds.Pool.Query(ctx, fmt.Sprintf(`SELECT status FROM %s.%s WHERE consumer_group_id = $1 AND message_id = $2 AND attempt = $3 ORDER BY id;`, ds.Schema, stream.DeliveryLogTable(streamId)), groupId, messageId, attempt)
	common.Must(err)
	defer rows.Close()
	var got []string
	for rows.Next() {
		var status string
		common.Must(rows.Scan(&status))
		got = append(got, status)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		common.Die(fmt.Sprintf("%s.%s[message=%d attempt=%d] statuses=%v, want %v", ds.Schema, stream.DeliveryLogTable(streamId), messageId, attempt, got, want))
	}
	fmt.Printf("  ✓ delivery_log_%d[message=%d attempt=%d] statuses=%v\n", streamId, messageId, attempt, got)
}

func assertDeliveryLogCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, groupId int64, messageId int64, want int) {
	var count int
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE consumer_group_id = $1 AND message_id = $2;`, ds.Schema, stream.DeliveryLogTable(streamId)), groupId, messageId).Scan(&count))
	if count != want {
		common.Die(fmt.Sprintf("%s.%s[message=%d] has %d rows, want %d", ds.Schema, stream.DeliveryLogTable(streamId), messageId, count, want))
	}
	fmt.Printf("  ✓ delivery_log_%d[message=%d] has %d row(s)\n", streamId, messageId, count)
}

func assertDeliveryRowCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, want int) {
	var count int
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s;`, ds.Schema, stream.ExceptionQueueTable(streamId))).Scan(&count))
	if count != want {
		common.Die(fmt.Sprintf("%s.%s has %d rows, want %d", ds.Schema, stream.ExceptionQueueTable(streamId), count, want))
	}
	fmt.Printf("  ✓ exception_queue_%d has %d row(s)\n", streamId, count)
}

func assertTableExists(ctx context.Context, ds *iDatastore.PostgresDatastore, table string, want bool) {
	var exists *string
	common.Must(ds.Pool.QueryRow(ctx, `SELECT to_regclass($1)::text;`, table).Scan(&exists))
	got := exists != nil
	if got != want {
		common.Die(fmt.Sprintf("%s exists=%v, want %v", table, got, want))
	}
	fmt.Printf("  ✓ %s exists=%v\n", table, got)
}

func step(s string)                                    { fmt.Printf("\n--- %s ---\n", s) }
func mustGroupID(g *consume.Consumer, err error) int64 { common.Must(err); return g.Id }
