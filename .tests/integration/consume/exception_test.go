package consume

import (
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/pkg/common"
	keyleasedatastore "github.com/agentstax/sqlstreams/pkg/consume/base/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5/pgtype"
)

// behavior: a claim takes a ready row once can_run_after has passed, an
// inflight row once its lease has expired, and a deferred row at any time;
// it leaves every other row alone.
func TestExceptionClaimTakesOnlyDueRows(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 8)
	insertException(t, groups, consumer, 1, "ready", -time.Hour, 0)
	insertException(t, groups, consumer, 2, "ready", time.Hour, 0)
	insertException(t, groups, consumer, 3, "inflight", 0, -time.Hour)
	insertException(t, groups, consumer, 4, "inflight", 0, time.Hour)
	insertException(t, groups, consumer, 5, "deferred", time.Hour, time.Hour)
	insertException(t, groups, consumer, 6, "done", -time.Hour, -time.Hour)
	insertException(t, groups, consumer, 7, "dead", -time.Hour, -time.Hour)
	insertException(t, groups, consumer, 8, "superseded", -time.Hour, -time.Hour)
	exceptions := newExceptionConsumerDatastore(t, groups)

	// test
	claimed, err := exceptions.Claim(t.Context(), consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, row := range claimed {
		ids = append(ids, row.MessageId)
	}
	if !reflect.DeepEqual(ids, []int64{1, 3, 5}) {
		t.Fatalf("Claim over one row per status = %v, want the due ready, expired inflight, and deferred rows [1 3 5]", ids)
	}
}

// invariant (one delivery per key): a claim leaves a row alone while another
// delivery holds its message key, and takes it once the key is released.
func TestExceptionClaimSkipsAKeyAnotherDeliveryHolds(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceKeyedMessages(t, groups, consumer, "order-1", 1)
	insertKeyedException(t, groups, consumer, 1, "order-1", common.ConcurrencyExclusive, -time.Hour)
	exceptions := newExceptionConsumerDatastore(t, groups)
	keys := newKeyLeaseDatastore(t, groups)
	ctx := t.Context()
	held, err := keys.Claim(ctx, consumer.StreamId, consumer.Id, "order-1", 1, false, common.ConcurrencyExclusive, 0, 0, time.Minute, pgtype.UUID{Bytes: uuid.New(), Valid: true})
	if err != nil || held.Verdict != keyleasedatastore.KeyLeaseAcquired {
		t.Fatalf("key lease Claim during setup = %+v, %v; want acquired", held, err)
	}

	// test
	claimed, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 0 {
		t.Fatalf("Claim while the key is held = %+v, want nothing", claimed)
	}

	// test: the key is released
	if _, err := keys.Release(ctx, held); err != nil {
		t.Fatal(err)
	}
	claimed, err = exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].MessageId != 1 {
		t.Fatalf("Claim after the key was released = %+v, want message 1", claimed)
	}
}

// invariant (per-key order): an ordered row is not claimed while an earlier
// same-key row is unresolved; the earlier row is claimed first.
func TestExceptionClaimHoldsAnOrderedRowBehindItsPredecessor(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceKeyedMessages(t, groups, consumer, "order-1", 2)
	insertKeyedException(t, groups, consumer, 1, "order-1", common.ConcurrencyOrdered, time.Hour)
	insertKeyedException(t, groups, consumer, 2, "order-1", common.ConcurrencyOrdered, -time.Hour)
	exceptions := newExceptionConsumerDatastore(t, groups)
	queue := groups.Datastore.Schema + "." + stream.ExceptionQueueTable(consumer.StreamId)
	ctx := t.Context()

	// test
	claimed, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 0 {
		t.Fatalf("Claim with the earlier same-key row not yet due = %+v, want nothing", claimed)
	}

	// test: the earlier row comes due
	if _, err := groups.Datastore.Pool.Exec(ctx, "UPDATE "+queue+" SET can_run_after = now() WHERE consumer_group_id = $1 AND message_id = 1", consumer.Id); err != nil {
		t.Fatal(err)
	}
	claimed, err = exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].MessageId != 1 {
		t.Fatalf("Claim once the earlier row is due = %+v, want message 1 alone", claimed)
	}
}

// behavior: taking over an inflight row whose lease expired writes one
// 'expired' delivery log row at the attempt that was abandoned, in the same
// statement as the claim.
func TestExceptionClaimLogsTheExpiredAttemptItTakesOver(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 1)
	insertException(t, groups, consumer, 1, "inflight", 0, -time.Hour)
	exceptions := newExceptionConsumerDatastore(t, groups)
	queue := groups.Datastore.Schema + "." + stream.ExceptionQueueTable(consumer.StreamId)
	logs := groups.Datastore.Schema + "." + stream.DeliveryLogTable(consumer.StreamId)
	ctx := t.Context()
	if _, err := groups.Datastore.Pool.Exec(ctx, "UPDATE "+queue+" SET attempts = 2 WHERE consumer_group_id = $1 AND message_id = 1", consumer.Id); err != nil {
		t.Fatal(err)
	}

	// test
	claimed, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeFailures)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].Attempts != 3 {
		t.Fatalf("Claim over an expired inflight row at attempt 2 = %+v, want the row at attempt 3", claimed)
	}
	var expired int
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE consumer_group_id = $1 AND message_id = 1 AND status = 'expired' AND attempt = 2", consumer.Id).Scan(&expired); err != nil {
		t.Fatal(err)
	}
	if expired != 1 {
		t.Fatalf("'expired' delivery log rows at attempt 2 after the takeover = %d, want 1", expired)
	}
}

// invariant (one live lease): every outcome verb refuses a lease token the
// row no longer holds with ErrLeaseLost, and writes neither the row nor a
// delivery log row -- a delivery whose lease was taken over cannot report.
func TestOutcomeVerbsWithStaleLeaseTokenAreLeaseLost(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 1)
	insertException(t, groups, consumer, 1, "ready", -time.Hour, 0)
	exceptions := newExceptionConsumerDatastore(t, groups)
	metrics := newMetricDatastore(t, groups)
	logs := groups.Datastore.Schema + "." + stream.DeliveryLogTable(consumer.StreamId)
	retry := (&common.RetryPolicy{}).WithDefaults()
	ctx := t.Context()
	claimed, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeAll)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("Claim during setup = %+v, %v; want the one row", claimed, err)
	}
	stale := claimed[0]
	stale.LeaseToken = pgtype.UUID{Bytes: uuid.New(), Valid: true}

	// test
	successErr := exceptions.RecordSuccess(ctx, &stale, stream.DeliveryLogModeAll, nil)
	failureErr := exceptions.RecordFailure(ctx, retry, &stale, errors.New("handler returned an error"), stream.DeliveryLogModeAll, nil)
	delayedErr := exceptions.RecordDelayed(ctx, time.Hour, &stale, errors.New("handler asked to run later"), stream.DeliveryLogModeAll, nil)
	terminalErr := exceptions.RecordTerminal(ctx, &stale, errors.New("handler returned a terminal error"), stream.DeliveryLogModeAll, nil)
	supersededErr := exceptions.RecordSuperseded(ctx, &stale, stream.DeliveryLogModeAll)
	deferredErr := exceptions.RecordDeferred(ctx, &stale, common.ConcurrencyExclusive, stream.DeliveryLogModeAll)

	// verify
	if !errors.Is(successErr, common.ErrLeaseLost) {
		t.Errorf("RecordSuccess with a stale token = %v, want ErrLeaseLost", successErr)
	}
	if !errors.Is(failureErr, common.ErrLeaseLost) {
		t.Errorf("RecordFailure with a stale token = %v, want ErrLeaseLost", failureErr)
	}
	if !errors.Is(delayedErr, common.ErrLeaseLost) {
		t.Errorf("RecordDelayed with a stale token = %v, want ErrLeaseLost", delayedErr)
	}
	if !errors.Is(terminalErr, common.ErrLeaseLost) {
		t.Errorf("RecordTerminal with a stale token = %v, want ErrLeaseLost", terminalErr)
	}
	if !errors.Is(supersededErr, common.ErrLeaseLost) {
		t.Errorf("RecordSuperseded with a stale token = %v, want ErrLeaseLost", supersededErr)
	}
	if !errors.Is(deferredErr, common.ErrLeaseLost) {
		t.Errorf("RecordDeferred with a stale token = %v, want ErrLeaseLost", deferredErr)
	}
	snapshot, err := metrics.ConsumerGroupSnapshot(ctx, consumer.StreamId, consumer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.InflightExceptions != 1 {
		t.Fatalf("inflight exceptions after the stale-token verbs = %d, want the claimed row still inflight, 1", snapshot.InflightExceptions)
	}
	var logged int
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE consumer_group_id = $1", consumer.Id).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 0 {
		t.Fatalf("delivery log rows after the stale-token verbs = %d, want 0", logged)
	}
}

// behavior: a success deletes the exception row, a failure returns it to
// 'ready' behind its backoff, and a terminal failure marks it 'dead'.
func TestOutcomeVerbsResolveTheClaimedRow(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 3)
	insertException(t, groups, consumer, 1, "ready", -time.Hour, 0)
	insertException(t, groups, consumer, 2, "ready", -time.Hour, 0)
	insertException(t, groups, consumer, 3, "ready", -time.Hour, 0)
	exceptions := newExceptionConsumerDatastore(t, groups)
	metrics := newMetricDatastore(t, groups)
	queue := groups.Datastore.Schema + "." + stream.ExceptionQueueTable(consumer.StreamId)
	retry := (&common.RetryPolicy{BaseDelay: time.Hour, MaxDelay: time.Hour}).WithDefaults()
	ctx := t.Context()
	claimed, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)
	if err != nil || len(claimed) != 3 {
		t.Fatalf("Claim during setup = %+v, %v; want the three rows", claimed, err)
	}

	// test
	successErr := exceptions.RecordSuccess(ctx, &claimed[0], stream.DeliveryLogModeOff, nil)
	failureErr := exceptions.RecordFailure(ctx, retry, &claimed[1], errors.New("handler returned an error"), stream.DeliveryLogModeOff, nil)
	terminalErr := exceptions.RecordTerminal(ctx, &claimed[2], errors.New("handler returned a terminal error"), stream.DeliveryLogModeOff, nil)

	// verify
	if successErr != nil || failureErr != nil || terminalErr != nil {
		t.Fatalf("RecordSuccess, RecordFailure, RecordTerminal = %v, %v, %v; want nil, nil, nil", successErr, failureErr, terminalErr)
	}
	snapshot, err := metrics.ConsumerGroupSnapshot(ctx, consumer.StreamId, consumer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ReadyExceptions != 1 || snapshot.DeadExceptions != 1 || snapshot.InflightExceptions != 0 {
		t.Fatalf("exceptions after the outcomes = (ready %d, dead %d, inflight %d), want (1, 1, 0)", snapshot.ReadyExceptions, snapshot.DeadExceptions, snapshot.InflightExceptions)
	}
	var remaining int
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+queue+" WHERE consumer_group_id = $1", consumer.Id).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 2 {
		t.Fatalf("exception rows after the outcomes = %d, want the success's row deleted, 2", remaining)
	}
	inBackoff, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)
	if err != nil {
		t.Fatal(err)
	}
	if len(inBackoff) != 0 {
		t.Fatalf("Claim right after the failure = %+v, want nothing, the failed row is behind its backoff", inBackoff)
	}

	// test: the backoff passes
	if _, err := groups.Datastore.Pool.Exec(ctx, "UPDATE "+queue+" SET can_run_after = now() WHERE consumer_group_id = $1", consumer.Id); err != nil {
		t.Fatal(err)
	}
	due, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].MessageId != 2 {
		t.Fatalf("Claim after the backoff = %+v, want the failed message 2 alone", due)
	}
}

// invariant (retry budget): a delay, a supersede, and a deferral are not
// failures -- a delay counts in delays beside attempts, the other two hand
// back the attempt the claim took -- and each logs at the claim's attempt.
func TestDelayedSupersededAndDeferredKeepTheRetryBudget(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 3)
	insertException(t, groups, consumer, 1, "ready", -time.Hour, 0)
	insertException(t, groups, consumer, 2, "ready", -time.Hour, 0)
	insertException(t, groups, consumer, 3, "ready", -time.Hour, 0)
	exceptions := newExceptionConsumerDatastore(t, groups)
	queue := groups.Datastore.Schema + "." + stream.ExceptionQueueTable(consumer.StreamId)
	logs := groups.Datastore.Schema + "." + stream.DeliveryLogTable(consumer.StreamId)
	ctx := t.Context()
	claimed, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeFailures)
	if err != nil || len(claimed) != 3 {
		t.Fatalf("Claim during setup = %+v, %v; want the three rows", claimed, err)
	}

	// test
	delayedErr := exceptions.RecordDelayed(ctx, time.Hour, &claimed[0], errors.New("handler asked to run later"), stream.DeliveryLogModeFailures, nil)
	supersededErr := exceptions.RecordSuperseded(ctx, &claimed[1], stream.DeliveryLogModeFailures)
	deferredErr := exceptions.RecordDeferred(ctx, &claimed[2], common.ConcurrencyExclusive, stream.DeliveryLogModeFailures)

	// verify: the claim took each row to attempt 1
	if delayedErr != nil || supersededErr != nil || deferredErr != nil {
		t.Fatalf("RecordDelayed, RecordSuperseded, RecordDeferred = %v, %v, %v; want nil, nil, nil", delayedErr, supersededErr, deferredErr)
	}
	var logged int
	if err := groups.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+logs+" WHERE consumer_group_id = $1 AND attempt = 1 AND (message_id, status) IN ((1, 'delayed'), (2, 'superseded'), (3, 'deferred'))", consumer.Id).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 3 {
		t.Fatalf("delivery log rows at attempt 1 with the outcome's status = %d, want 3", logged)
	}
	deferred, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeFailures)
	if err != nil {
		t.Fatal(err)
	}
	if len(deferred) != 1 || deferred[0].MessageId != 3 || deferred[0].Attempts != 1 {
		t.Fatalf("Claim after the outcomes = %+v, want the deferred message 3 back at attempt 1", deferred)
	}

	// test: the delay passes
	if _, err := groups.Datastore.Pool.Exec(ctx, "UPDATE "+queue+" SET can_run_after = now() WHERE consumer_group_id = $1", consumer.Id); err != nil {
		t.Fatal(err)
	}
	delayed, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeFailures)

	// verify: the superseded row is never claimed again
	if err != nil {
		t.Fatal(err)
	}
	if len(delayed) != 1 || delayed[0].MessageId != 1 || delayed[0].Attempts != 2 || delayed[0].Delays != 1 {
		t.Fatalf("Claim after the delay = %+v, want the delayed message 1 at attempt 2 with 1 delay", delayed)
	}
}

// invariant (crash-loop backstop): the kill marks 'dead' only an inflight
// row whose lease has expired at or over its retry budget, and returns how
// many; a live lease or an unspent budget keeps the row.
func TestKillDeadsOnlyExpiredInflightRowsOverBudget(t *testing.T) {
	// setup
	groups, consumer := newMessageConsumerDatastore(t)
	produceMessages(t, groups, consumer, 3)
	insertException(t, groups, consumer, 1, "inflight", 0, -time.Hour)
	insertException(t, groups, consumer, 2, "inflight", 0, time.Hour)
	insertException(t, groups, consumer, 3, "inflight", 0, -time.Hour)
	exceptions := newExceptionConsumerDatastore(t, groups)
	metrics := newMetricDatastore(t, groups)
	queue := groups.Datastore.Schema + "." + stream.ExceptionQueueTable(consumer.StreamId)
	ctx := t.Context()
	if _, err := groups.Datastore.Pool.Exec(ctx, "UPDATE "+queue+" SET attempts = 3 WHERE consumer_group_id = $1 AND message_id IN (1, 2)", consumer.Id); err != nil {
		t.Fatal(err)
	}

	// test
	killed, err := exceptions.Kill(ctx, consumer.StreamId, consumer.Id, 3, stream.DeliveryLogModeOff)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if killed != 1 {
		t.Fatalf("Kill(maxRetries 3) = %d, want 1", killed)
	}
	snapshot, err := metrics.ConsumerGroupSnapshot(ctx, consumer.StreamId, consumer.Id)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.DeadExceptions != 1 || snapshot.InflightExceptions != 2 {
		t.Fatalf("exceptions after Kill = (dead %d, inflight %d), want (1, 2)", snapshot.DeadExceptions, snapshot.InflightExceptions)
	}
	claimed, err := exceptions.Claim(ctx, consumer.StreamId, consumer.Id, 1, 100, 10, time.Minute, stream.DeliveryLogModeOff)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].MessageId != 3 {
		t.Fatalf("Claim after Kill = %+v, want only the expired row under budget, message 3", claimed)
	}
}
