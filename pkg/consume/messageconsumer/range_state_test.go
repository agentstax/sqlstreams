package messageconsumer

import (
	"errors"
	"reflect"
	"testing"
	"uuid"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume/messageconsumer/controller"
)

// invariant: a partial commit only advances the watermark over a contiguous
// run of resolved messages, so it can never skip past work still in flight.
func TestContiguousResolvedStopsAtTheLowestUnresolvedRow(t *testing.T) {
	// setup
	state := newRangeState(claimedRange(t, 99, []int64{100, 101, 102, 103, 104}, "", ""), false)

	// test
	state.resolve(0, kindException, "boom", 0, common.ConcurrencyParallel)
	state.resolve(2, kindException, "boom", 0, common.ConcurrencyParallel)
	state.resolve(3, kindSuccess, "", 0, common.ConcurrencyParallel)
	state.resolve(4, kindTerminal, "dead", 0, common.ConcurrencyParallel)
	lastProcessed, outcomes := state.contiguousResolved()

	// verify
	if lastProcessed != 100 {
		t.Errorf("contiguousResolved() lastProcessed = %d, want %d", lastProcessed, 100)
	}
	want := []controller.MessageOutcome{{MessageId: 100, Concurrency: common.ConcurrencyParallel, Kind: controller.OutcomeException, Err: "boom"}}
	if !reflect.DeepEqual(outcomes, want) {
		t.Errorf("contiguousResolved() outcomes = %+v, want %+v", outcomes, want)
	}
	if state.isResolved() {
		t.Errorf("isResolved() = true, want false with index 1 pending")
	}
}

// invariant: a range with nothing resolved commits at the lease's low bound
// and carries no outcomes.
func TestContiguousResolvedOfAnUntouchedRangeStaysAtTheLowBound(t *testing.T) {
	// setup
	state := newRangeState(claimedRange(t, 99, []int64{100, 101}, "", ""), false)

	// test
	lastProcessed, outcomes := state.contiguousResolved()

	// verify
	if lastProcessed != 99 {
		t.Errorf("contiguousResolved() lastProcessed = %d, want %d", lastProcessed, 99)
	}
	if len(outcomes) != 0 {
		t.Errorf("contiguousResolved() outcomes = %+v, want none", outcomes)
	}
}

// invariant: the snapshot is handed to exactly one caller, and a call before
// the range is resolved does not spend that one ownership.
func TestSnapshotIsHandedToExactlyOneCaller(t *testing.T) {
	// setup
	state := newRangeState(claimedRange(t, 0, []int64{1, 2}, "", ""), false)
	state.resolve(0, kindSuccess, "", 0, common.ConcurrencyParallel)

	// test
	_, earlyErr := state.tryGetSnapshot()
	state.resolve(1, kindException, "boom", 0, common.ConcurrencyParallel)
	first, firstErr := state.tryGetSnapshot()
	second, secondErr := state.tryGetSnapshot()

	// verify
	if !errors.Is(earlyErr, errRangeNotResolved) {
		t.Errorf("tryGetSnapshot() before resolve err = %v, want %v", earlyErr, errRangeNotResolved)
	}
	if firstErr != nil || first == nil {
		t.Fatalf("tryGetSnapshot() first = %+v, %v, want a snapshot", first, firstErr)
	}
	if second != nil || !errors.Is(secondErr, errSnapshotTaken) {
		t.Errorf("tryGetSnapshot() second = %+v, %v, want nil, %v", second, secondErr, errSnapshotTaken)
	}
	if first.Lease.Low != 0 || first.Lease.High != 2 {
		t.Errorf("snapshot lease = %+v, want low 0 high 2", first.Lease)
	}
	if len(first.Outcomes) != 1 || first.Outcomes[0].MessageId != 2 {
		t.Errorf("snapshot outcomes = %+v, want only message 2", first.Outcomes)
	}
}

// behavior: success outcomes reach the commit only when the range was
// built to include them (DeliveryLogModeAll); every other kind always does.
func TestSuccessOutcomesAreCollectedOnlyWhenIncluded(t *testing.T) {
	// setup
	claimed := claimedRange(t, 0, []int64{1, 2}, "", "")
	omitting := newRangeState(claimed, false)
	including := newRangeState(claimed, true)

	// test
	omitting.resolve(0, kindSuccess, "", 0, common.ConcurrencyParallel)
	omitting.resolve(1, kindException, "boom", 0, common.ConcurrencyParallel)
	including.resolve(0, kindSuccess, "", 0, common.ConcurrencyParallel)
	including.resolve(1, kindException, "boom", 0, common.ConcurrencyParallel)
	omitted := omitting.resolvedOutcomes()
	included := including.resolvedOutcomes()

	// verify
	if len(omitted) != 1 || omitted[0].Kind != controller.OutcomeException {
		t.Errorf("resolvedOutcomes() omitting successes = %+v, want the one exception", omitted)
	}
	if len(included) != 2 || included[0].Kind != controller.OutcomeSuccess || included[1].Kind != controller.OutcomeException {
		t.Errorf("resolvedOutcomes() including successes = %+v, want success then exception", included)
	}
}

// ***************
// *** HELPERS ***
// ***************

// claimedRange builds a claimed range whose lease runs (low, last id]; every
// message carries key and concurrency, "" for none.
func claimedRange(t testing.TB, low int64, ids []int64, key string, concurrency common.ConcurrencyPolicy) *controller.ClaimedRange {
	t.Helper()

	messages := make([]controller.Message, len(ids))
	for i, id := range ids {
		messages[i] = controller.Message{Id: id, MessageKey: key, Options: &common.MessageOptions{Concurrency: concurrency}}
	}
	return &controller.ClaimedRange{
		Lease:    controller.RangeLease{Token: uuid.NewV7(), Low: low, High: ids[len(ids)-1]},
		Messages: messages,
	}
}
