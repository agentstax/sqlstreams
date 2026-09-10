package schedule

import (
	"reflect"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/schedule/controller/datastore"
)

// behavior: a status lists every group that receives the schedule's messages
// -- no bindings, or a binding matching the name -- and counts, per group, a
// message with a success row as succeeded, one with a failure row as failed,
// an older unrun message the head replaced as superseded, and the pending
// head as nothing.
func TestStatusCountsOutcomesPerReceivingGroup(t *testing.T) {
	// setup
	schedules, orders := newScheduleDatastore(t)
	ctx := t.Context()
	registerSchedule(t, schedules, orders, "nightly", "@hourly")
	all := registerConsumerGroup(t, schedules, orders, "all", nil)
	bound := registerConsumerGroup(t, schedules, orders, "bound", []string{"nightly"})
	registerConsumerGroup(t, schedules, orders, "billing", []string{"billing.*"})
	first := produceScheduleMessage(t, schedules, orders, "nightly", time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC))
	second := produceScheduleMessage(t, schedules, orders, "nightly", time.Date(2026, 9, 10, 2, 0, 0, 0, time.UTC))
	produceScheduleMessage(t, schedules, orders, "nightly", time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC))
	produceScheduleMessage(t, schedules, orders, "nightly", time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC))
	insertDeliveryOutcome(t, schedules, orders, all.Id, first, "success")
	insertDeliveryOutcome(t, schedules, orders, all.Id, second, "failure")
	insertDeliveryOutcome(t, schedules, orders, bound.Id, first, "expired")

	// test
	statuses, err := schedules.Status(ctx, orders.Id, "nightly")

	// verify
	if err != nil {
		t.Fatal(err)
	}
	want := []datastore.ScheduleConsumerGroupSummaryRow{
		{ConsumerGroup: "all", Ran: 2, Succeeded: 1, Superseded: 1, Failed: 1},
		{ConsumerGroup: "bound", Ran: 1, Succeeded: 0, Superseded: 2, Failed: 1},
	}
	if !reflect.DeepEqual(statuses, want) {
		t.Errorf("Status(nightly) = %+v, want %+v", statuses, want)
	}
}

// behavior: a message listing returns the newest messages within the limit,
// newest first, each with the scheduled time read from its stored options,
// the head flagged, and the group's outcome booleans.
func TestListMessagesReadsTheNewestMessagesWithTheirScheduledTimes(t *testing.T) {
	// setup
	schedules, orders := newScheduleDatastore(t)
	ctx := t.Context()
	registerSchedule(t, schedules, orders, "nightly", "@hourly")
	all := registerConsumerGroup(t, schedules, orders, "all", nil)
	first := produceScheduleMessage(t, schedules, orders, "nightly", time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC))
	second := produceScheduleMessage(t, schedules, orders, "nightly", time.Date(2026, 9, 10, 2, 0, 0, 0, time.UTC))
	third := produceScheduleMessage(t, schedules, orders, "nightly", time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC))
	insertDeliveryOutcome(t, schedules, orders, all.Id, first, "success")
	insertDeliveryOutcome(t, schedules, orders, all.Id, second, "deferred")

	// test
	messages, err := schedules.ListMessages(ctx, orders.Id, "nightly", 2)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("ListMessages(nightly, limit 2) = %+v, want two rows", messages)
	}
	head := messages[0]
	if head.ConsumerGroup != "all" || head.MessageId != third || !head.Head || !head.ScheduledAt.Equal(time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC)) {
		t.Errorf("ListMessages(nightly, limit 2)[0] = %+v, want the head, message %d for all, scheduled 03:00", head, third)
	}
	deferred := messages[1]
	if deferred.MessageId != second || deferred.Head || !deferred.Deferred || deferred.Succeeded || !deferred.ScheduledAt.Equal(time.Date(2026, 9, 10, 2, 0, 0, 0, time.UTC)) {
		t.Errorf("ListMessages(nightly, limit 2)[1] = %+v, want message %d deferred, not the head, scheduled 02:00", deferred, second)
	}
}
