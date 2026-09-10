package schedule

import (
	"slices"
	"testing"
	"time"
)

// behavior: the due scan lists every unsuspended schedule at or past its next
// scheduled time, earliest due first, and leaves out a future one and a
// suspended one that is due.
func TestListDueOrdersDueUnsuspendedSchedulesByTheirTime(t *testing.T) {
	// setup
	schedules, orders := newScheduleDatastore(t)
	ctx := t.Context()
	producers := newScheduleProducerDatastore(t, schedules)
	later := registerSchedule(t, schedules, orders, "later", "@hourly")
	earlier := registerSchedule(t, schedules, orders, "earlier", "@hourly")
	future := registerSchedule(t, schedules, orders, "future", "@hourly")
	paused := registerSchedule(t, schedules, orders, "paused", "@hourly")
	setNextScheduledAt(t, schedules, later.Id, -time.Hour)
	setNextScheduledAt(t, schedules, earlier.Id, -2*time.Hour)
	setNextScheduledAt(t, schedules, future.Id, time.Hour)
	setNextScheduledAt(t, schedules, paused.Id, -time.Hour)
	if err := schedules.Suspend(ctx, "paused"); err != nil {
		t.Fatal(err)
	}

	// test
	due, err := producers.ListDue(ctx)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(due, []int64{earlier.Id, later.Id}) {
		t.Errorf("ListDue() = %v, want the due schedules earliest first [%d %d]", due, earlier.Id, later.Id)
	}
}

// invariant (one producer per due time): a claim inside a transaction locks
// the due row; another transaction's claim on it is nil instead of waiting;
// once the holder ends the row can be claimed again; a not-due or suspended
// row claims nil; the advance in the claiming transaction takes the row out
// of the due scan and records the produced time.
func TestClaimDueLocksTheRowUntilItsTransactionEnds(t *testing.T) {
	// setup
	schedules, orders := newScheduleDatastore(t)
	ctx := t.Context()
	producers := newScheduleProducerDatastore(t, schedules)
	nightly := registerSchedule(t, schedules, orders, "nightly", "@hourly")
	future := registerSchedule(t, schedules, orders, "future", "@hourly")
	paused := registerSchedule(t, schedules, orders, "paused", "@hourly")
	setNextScheduledAt(t, schedules, nightly.Id, -time.Hour)
	setNextScheduledAt(t, schedules, paused.Id, -time.Hour)
	if err := schedules.Suspend(ctx, "paused"); err != nil {
		t.Fatal(err)
	}
	holder := holdTransaction(t, schedules.Datastore.Pool)
	other := holdTransaction(t, schedules.Datastore.Pool)

	// test
	claimed, claimErr := producers.ClaimDue(ctx, holder, nightly.Id)
	racing, racingErr := producers.ClaimDue(ctx, other, nightly.Id)
	notDue, notDueErr := producers.ClaimDue(ctx, other, future.Id)
	suspended, suspendedErr := producers.ClaimDue(ctx, other, paused.Id)

	// verify
	if claimErr != nil {
		t.Fatal(claimErr)
	}
	if claimed == nil || claimed.Id != nightly.Id || claimed.Name != "nightly" || claimed.StreamName != "orders" || claimed.DbNow.IsZero() {
		t.Fatalf("ClaimDue(nightly) in the holder = %+v, want the row with its stream name and the database clock", claimed)
	}
	if racing != nil || racingErr != nil {
		t.Errorf("ClaimDue(nightly) from another transaction while held = %+v, %v; want nil, nil", racing, racingErr)
	}
	if notDue != nil || notDueErr != nil {
		t.Errorf("ClaimDue(future) = %+v, %v; want nil, nil", notDue, notDueErr)
	}
	if suspended != nil || suspendedErr != nil {
		t.Errorf("ClaimDue(paused) = %+v, %v; want nil, nil", suspended, suspendedErr)
	}

	// test: the holder ends without producing, a second producer claims and advances
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	reclaimed, reclaimErr := producers.ClaimDue(ctx, other, nightly.Id)
	if reclaimErr != nil || reclaimed == nil {
		t.Fatalf("ClaimDue(nightly) after the holder rolled back = %+v, %v; want the row", reclaimed, reclaimErr)
	}
	next := reclaimed.DbNow.Add(time.Hour)
	advanceErr := producers.Advance(ctx, other, nightly.Id, next, reclaimed.NextScheduledAt)
	if err := other.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	due, dueErr := producers.ListDue(ctx)
	advanced, getErr := schedules.Get(ctx, "nightly")

	// verify
	if advanceErr != nil {
		t.Fatal(advanceErr)
	}
	if dueErr != nil {
		t.Fatal(dueErr)
	}
	if len(due) != 0 {
		t.Errorf("ListDue() after the advance = %v, want nothing due", due)
	}
	if getErr != nil {
		t.Fatal(getErr)
	}
	if !advanced.NextScheduledAt.Equal(next) || advanced.LastScheduledAt == nil || !advanced.LastScheduledAt.Equal(reclaimed.NextScheduledAt) {
		t.Errorf("Get(nightly) after the advance = %+v, want next scheduled at %v and last scheduled at %v", advanced, next, reclaimed.NextScheduledAt)
	}
}
