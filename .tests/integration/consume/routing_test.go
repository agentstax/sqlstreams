package consume

import (
	"testing"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// invariant (routing): a group's bindings are evaluated when its messages
// are read, not when they are produced -- a message produced before the
// binding existed still matches, a wildcard crosses hierarchy depth, an
// unbound group reads everything, and the cursor path's lease still covers
// the messages it filtered out.
func TestBindingsFilterAtClaimTimeOnBothPaths(t *testing.T) {
	// setup: message 1 is produced before any binding exists
	groups, consumer := newMessageConsumerDatastore(t)
	consumers := newConsumeDatastore(t, groups)
	produceRoutedMessage(t, groups, consumer, "orders.us.created")
	unbound := registerConsumer(t, groups, consumer, "auditors")
	lifecycle := registerConsumer(t, groups, consumer, "billing")
	deliveries := newDeliveryConsumerDatastore(t, groups)
	ctx := t.Context()
	if _, err := consumers.DeclareBindings(ctx, consumer.StreamId, consumer.Id, []string{"orders.*.created"}, "host-a", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := consumers.DeclareBindings(ctx, lifecycle.StreamId, lifecycle.Id, []string{"payments.*"}, "host-a", time.Now()); err != nil {
		t.Fatal(err)
	}
	produceRoutedMessage(t, groups, consumer, "orders.us.central1.created")
	produceRoutedMessage(t, groups, consumer, "orders.eu.updated")
	produceRoutedMessage(t, groups, consumer, "payments.charge")
	produceRoutedMessage(t, groups, consumer, "")

	// test
	bound, boundErr := groups.ClaimMessagesWithCursor(ctx, consumer.StreamId, consumer.Id, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)
	everything, everythingErr := groups.ClaimMessagesWithCursor(ctx, unbound.StreamId, unbound.Id, 1, 100, 3, time.Minute, stream.DeliveryLogModeFailures)

	// verify
	if boundErr != nil || everythingErr != nil {
		t.Fatalf("ClaimMessagesWithCursor for the bound and the unbound group = %v, %v; want nil, nil", boundErr, everythingErr)
	}
	if bound == nil || bound.Lease.Low != 0 || bound.Lease.High != 5 {
		t.Fatalf("bound group's claim = %+v, want lease (0, 5] over the whole range", bound)
	}
	if ids := messageIds(bound.Messages); len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Errorf("bound group's messages = %v, want the two orders.*.created matches [1 2]", ids)
	}
	if everything == nil || len(everything.Messages) != 5 {
		t.Errorf("unbound group's claim = %+v, want every message", everything)
	}

	// test: the lifecycle path
	if err := deliveries.FanOut(ctx, lifecycle.StreamId, lifecycle.Id, 1, 100); err != nil {
		t.Fatal(err)
	}
	delivered, err := deliveries.ClaimMessagesWithLifecycle(ctx, lifecycle.StreamId, lifecycle.Id, 100)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(delivered) != 1 || delivered[0].MessageId != 4 {
		t.Errorf("ClaimMessagesWithLifecycle after FanOut = %+v, want a delivery for payments.charge alone [4]", delivered)
	}
}
