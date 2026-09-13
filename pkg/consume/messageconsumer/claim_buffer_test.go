package messageconsumer

import (
	"context"
	"errors"
	"testing"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/concurrency"
)

// invariant: per-key order -- an ordered key's messages queue as one chain
// whose head alone is dispatched, so a later same-key message never runs
// before the one ahead of it resolves.
func TestAddQueuesOnlyTheHeadOfAnOrderedKeyChain(t *testing.T) {
	// setup
	buffer, cfg := newTestClaimBuffer(t, 8)
	claimed := claimedRange(t, 0, []int64{1, 2, 3}, "order-42", common.ConcurrencyOrdered)

	// test
	if err := buffer.add(context.Background(), claimed, cfg); err != nil {
		t.Fatalf("add() err = %v, want nil", err)
	}
	head, err := buffer.waitForNext(context.Background())
	if err != nil {
		t.Fatalf("waitForNext() err = %v, want nil", err)
	}
	room, err := buffer.waitForRoom(context.Background(), 0, 8)

	// verify
	if head.row.Id != 1 {
		t.Errorf("waitForNext() id = %d, want %d", head.row.Id, 1)
	}
	if err != nil || room != 8 {
		t.Errorf("waitForRoom() = %d, %v, want 8, nil with the chain's tail unqueued", room, err)
	}
	if head.next == nil || head.next.row.Id != 2 || head.next.next == nil || head.next.next.row.Id != 3 || head.next.next.next != nil {
		t.Errorf("chain behind head = %+v, want ids 2 then 3", head.next)
	}
}

// invariant: a failed ordered message defers everything chained behind it,
// which resolves the range without dispatching the deferred messages.
func TestResolveDeferredBehindDefersTheWholeChain(t *testing.T) {
	// setup
	buffer, cfg := newTestClaimBuffer(t, 8)
	claimed := claimedRange(t, 0, []int64{1, 2, 3}, "order-42", common.ConcurrencyOrdered)
	if err := buffer.add(context.Background(), claimed, cfg); err != nil {
		t.Fatalf("add() err = %v, want nil", err)
	}
	head, err := buffer.waitForNext(context.Background())
	if err != nil {
		t.Fatalf("waitForNext() err = %v, want nil", err)
	}

	// test
	buffer.resolveException(head, errors.New("boom"))
	buffer.resolveDeferredBehind(head)
	snapshot, err := buffer.tryGetRangeSnapshot(claimed.Lease.Token)

	// verify
	if err != nil {
		t.Fatalf("tryGetRangeSnapshot() err = %v, want nil", err)
	}
	if len(snapshot.Outcomes) != 3 {
		t.Fatalf("snapshot outcomes = %+v, want 3", snapshot.Outcomes)
	}
	if kind := snapshot.Outcomes[0].Kind; kind != "exception" {
		t.Errorf("outcome[0].Kind = %q, want exception", kind)
	}
	for _, outcome := range snapshot.Outcomes[1:] {
		if outcome.Kind != "deferred" {
			t.Errorf("outcome for message %d kind = %q, want deferred", outcome.MessageId, outcome.Kind)
		}
	}
}

// invariant: the shutdown fence -- removeAll hands every open range to the
// caller in one step, and a resolver that arrives after it changes nothing.
func TestRemoveAllFencesLateResolvers(t *testing.T) {
	// setup
	buffer, cfg := newTestClaimBuffer(t, 8)
	first := claimedRange(t, 0, []int64{1, 2}, "", "")
	second := claimedRange(t, 2, []int64{3}, "", "")
	if err := buffer.add(context.Background(), first, cfg); err != nil {
		t.Fatalf("add(first) err = %v, want nil", err)
	}
	if err := buffer.add(context.Background(), second, cfg); err != nil {
		t.Fatalf("add(second) err = %v, want nil", err)
	}
	item, err := buffer.waitForNext(context.Background())
	if err != nil {
		t.Fatalf("waitForNext() err = %v, want nil", err)
	}

	// test
	states := buffer.removeAll()
	buffer.resolveSuccess(item)
	_, outcomeKnown := buffer.outcomeOf(item)
	_, snapshotErr := buffer.tryGetRangeSnapshot(first.Lease.Token)
	remaining := buffer.removeAll()

	// verify
	if len(states) != 2 {
		t.Errorf("removeAll() = %d ranges, want 2", len(states))
	}
	if outcomeKnown {
		t.Errorf("outcomeOf() after removeAll = known, want unknown")
	}
	if !errors.Is(snapshotErr, errRangeNotTracked) {
		t.Errorf("tryGetRangeSnapshot() after removeAll err = %v, want %v", snapshotErr, errRangeNotTracked)
	}
	if len(remaining) != 0 {
		t.Errorf("second removeAll() = %d ranges, want 0", len(remaining))
	}
	for _, state := range states {
		if state.isResolved() {
			t.Errorf("range %v resolved after the fence, want untouched", state.lease.Token)
		}
	}
}

// behavior: neverDispatched answers whether any message of the range left
// the queue, which is what decides force reclaim against partial commit at
// shutdown.
func TestNeverDispatchedFlipsOnTheFirstDequeue(t *testing.T) {
	// setup
	buffer, cfg := newTestClaimBuffer(t, 8)
	claimed := claimedRange(t, 0, []int64{1, 2}, "", "")
	if err := buffer.add(context.Background(), claimed, cfg); err != nil {
		t.Fatalf("add() err = %v, want nil", err)
	}

	// test
	before := buffer.lookup(claimed.Lease.Token).neverDispatched()
	if _, err := buffer.waitForNext(context.Background()); err != nil {
		t.Fatalf("waitForNext() err = %v, want nil", err)
	}
	after := buffer.lookup(claimed.Lease.Token).neverDispatched()

	// verify
	if !before {
		t.Errorf("neverDispatched() before dequeue = false, want true")
	}
	if after {
		t.Errorf("neverDispatched() after dequeue = true, want false")
	}
}

// invariant: a range is marked stale once, so the stale-range warning is
// emitted once per range however many queued messages observe it.
func TestMarkStaleRecordsOnce(t *testing.T) {
	// setup
	buffer, cfg := newTestClaimBuffer(t, 8)
	claimed := claimedRange(t, 0, []int64{1, 2}, "", "")
	if err := buffer.add(context.Background(), claimed, cfg); err != nil {
		t.Fatalf("add() err = %v, want nil", err)
	}

	// test
	first := buffer.markStale(claimed.Lease.Token)
	second := buffer.markStale(claimed.Lease.Token)

	// verify
	if !first {
		t.Errorf("markStale() first = false, want true")
	}
	if second {
		t.Errorf("markStale() second = true, want false")
	}
}

// ***************
// *** HELPERS ***
// ***************

func newTestClaimBuffer(t testing.TB, queueSize int) (*claimBuffer, *MessageConsumerConfig) {
	t.Helper()

	queue, err := concurrency.NewPressureQueue[buffered](queueSize)
	if err != nil {
		t.Fatalf("NewPressureQueue(%d) err = %v, want nil", queueSize, err)
	}
	buffer, err := newClaimBuffer(queue, false)
	if err != nil {
		t.Fatalf("newClaimBuffer() err = %v, want nil", err)
	}
	cfg := (&MessageConsumerConfig{}).WithDefaults()
	return buffer, cfg
}
