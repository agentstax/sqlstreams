package schedule

import (
	"errors"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/schedule"
)

// invariant (crash consistency): a registration writes the schedule_config
// row and its schedule_cursor row together, the cursor at the expression's
// first time after the database clock -- a registration whose cursor cannot
// be written leaves no config row behind, since a config row without a
// cursor is invisible to Get yet holds the name.
func TestRegisterWritesTheConfigAndCursorRowsTogether(t *testing.T) {
	// setup
	schedules, orders := newScheduleDatastore(t)
	ctx := t.Context()
	configs := schedules.Datastore.Schema + ".schedule_config"

	// test
	nightly := registerSchedule(t, schedules, orders, "nightly", "@hourly")

	// verify
	if !nightly.NextScheduledAt.After(time.Now()) {
		t.Errorf("Register(nightly, @hourly).NextScheduledAt = %v, want after now", nightly.NextScheduledAt)
	}
	if nightly.LastScheduledAt != nil {
		t.Errorf("Register(nightly, @hourly).LastScheduledAt = %v, want nil before the first produce", *nightly.LastScheduledAt)
	}
	if nightly.Suspended {
		t.Error("Register(nightly, @hourly).Suspended = true, want false")
	}
	if string(nightly.Metadata) != "{}" {
		t.Errorf("Register(nightly, @hourly).Metadata = %s, want {} when none was declared", nightly.Metadata)
	}

	// test: the cursor insert fails
	rejectCursorInserts(t, schedules)
	parsed, err := schedule.ParseExpression("@daily")
	if err != nil {
		t.Fatal(err)
	}
	interrupted, err := schedules.Register(ctx, orders.SystemId, orders.Id, "weekly", parsed, common.ConcurrencyParallel, time.Minute, scheduleTestMessage{Kind: "weekly"}, 1, nil)

	// verify
	if err == nil {
		t.Fatalf("Register(weekly) with schedule_cursor rejecting inserts = %+v, want an error", interrupted)
	}
	var configRows int
	if err := schedules.Datastore.Pool.QueryRow(ctx, "SELECT count(*) FROM "+configs+" WHERE name = 'weekly'").Scan(&configRows); err != nil {
		t.Fatal(err)
	}
	if configRows != 0 {
		t.Errorf("schedule_config rows named weekly after the interrupted registration = %d, want 0", configRows)
	}
}

// behavior (newest declaration wins): registering again with the same values
// returns the stored row; a changed timeout under the same expression keeps
// the next scheduled time, so a due time is not dropped; a changed expression
// re-seeds the next scheduled time after the database clock.
func TestRegisterAgainReseedsTheCursorOnlyForANewExpression(t *testing.T) {
	// setup
	schedules, orders := newScheduleDatastore(t)
	ctx := t.Context()
	created := registerSchedule(t, schedules, orders, "nightly", "@hourly")
	setNextScheduledAt(t, schedules, created.Id, -time.Hour)
	stored, err := schedules.Get(ctx, "nightly")
	if err != nil {
		t.Fatal(err)
	}
	due := stored.NextScheduledAt
	hourly, err := schedule.ParseExpression("@hourly")
	if err != nil {
		t.Fatal(err)
	}
	daily, err := schedule.ParseExpression("@daily")
	if err != nil {
		t.Fatal(err)
	}

	// test
	same, sameErr := schedules.Register(ctx, orders.SystemId, orders.Id, "nightly", hourly, common.ConcurrencyParallel, time.Minute, scheduleTestMessage{Kind: "nightly"}, 1, nil)
	retimed, retimedErr := schedules.Register(ctx, orders.SystemId, orders.Id, "nightly", hourly, common.ConcurrencyParallel, 2*time.Minute, scheduleTestMessage{Kind: "nightly"}, 1, nil)
	reseeded, reseededErr := schedules.Register(ctx, orders.SystemId, orders.Id, "nightly", daily, common.ConcurrencyParallel, 2*time.Minute, scheduleTestMessage{Kind: "nightly"}, 1, nil)

	// verify
	if sameErr != nil {
		t.Fatal(sameErr)
	}
	if same.Id != created.Id || same.TimeoutNs != int64(time.Minute) || !same.NextScheduledAt.Equal(due) {
		t.Errorf("Register(nightly, same values) = %+v, want id %d, timeout 1m, next scheduled at %v", same, created.Id, due)
	}
	if retimedErr != nil {
		t.Fatal(retimedErr)
	}
	if retimed.TimeoutNs != int64(2*time.Minute) || !retimed.NextScheduledAt.Equal(due) {
		t.Errorf("Register(nightly, timeout 2m) = %+v, want timeout 2m with the due time %v kept", retimed, due)
	}
	if reseededErr != nil {
		t.Fatal(reseededErr)
	}
	if reseeded.Expression != "@daily" || !reseeded.NextScheduledAt.After(time.Now()) {
		t.Errorf("Register(nightly, @daily) = %+v, want expression @daily with the next scheduled time after now", reseeded)
	}
	final, err := schedules.Get(ctx, "nightly")
	if err != nil {
		t.Fatal(err)
	}
	if final.Id != created.Id || final.Expression != "@daily" || final.TimeoutNs != int64(2*time.Minute) || !final.NextScheduledAt.Equal(reseeded.NextScheduledAt) {
		t.Errorf("Get(nightly) after the three registrations = %+v, want id %d, @daily, timeout 2m, next scheduled at %v", final, created.Id, reseeded.NextScheduledAt)
	}
}

// behavior: a suspend sets the row suspended; an unsuspend clears it and
// moves a next scheduled time that came due while suspended past the
// database clock, so nothing is produced late; both are ErrScheduleNotFound
// for a name with no row.
func TestUnsuspendDropsTheTimeThatCameDueWhileSuspended(t *testing.T) {
	// setup
	schedules, orders := newScheduleDatastore(t)
	ctx := t.Context()
	nightly := registerSchedule(t, schedules, orders, "nightly", "@hourly")
	setNextScheduledAt(t, schedules, nightly.Id, -time.Hour)

	// test
	suspendErr := schedules.Suspend(ctx, "nightly")
	suspended, suspendedErr := schedules.Get(ctx, "nightly")
	unsuspendErr := schedules.Unsuspend(ctx, "nightly")
	resumed, resumedErr := schedules.Get(ctx, "nightly")
	missingSuspendErr := schedules.Suspend(ctx, "weekly")
	missingUnsuspendErr := schedules.Unsuspend(ctx, "weekly")

	// verify
	if suspendErr != nil || suspendedErr != nil {
		t.Fatalf("Suspend(nightly), Get(nightly) = %v, %v; want nil, nil", suspendErr, suspendedErr)
	}
	if !suspended.Suspended || !suspended.NextScheduledAt.Before(time.Now()) {
		t.Errorf("Get(nightly) after Suspend = %+v, want suspended with the due time kept", suspended)
	}
	if unsuspendErr != nil || resumedErr != nil {
		t.Fatalf("Unsuspend(nightly), Get(nightly) = %v, %v; want nil, nil", unsuspendErr, resumedErr)
	}
	if resumed.Suspended || !resumed.NextScheduledAt.After(time.Now()) {
		t.Errorf("Get(nightly) after Unsuspend = %+v, want not suspended with the next scheduled time after now", resumed)
	}
	if !errors.Is(missingSuspendErr, schedule.ErrScheduleNotFound) {
		t.Errorf("Suspend(weekly) = %v, want ErrScheduleNotFound", missingSuspendErr)
	}
	if !errors.Is(missingUnsuspendErr, schedule.ErrScheduleNotFound) {
		t.Errorf("Unsuspend(weekly) = %v, want ErrScheduleNotFound", missingUnsuspendErr)
	}
}
