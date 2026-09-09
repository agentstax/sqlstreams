package messageconsumer

import (
	"sync"
	"sync/atomic"
	"testing"
	"uuid"

	"github.com/agentstax/sqlstreams/pkg/common/concurrency"
	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller"
)

func TestStaleRangeHasOneWarningWinner(t *testing.T) {
	queue, err := concurrency.NewPressureQueue[buffered](4)
	if err != nil {
		t.Fatal(err)
	}
	buffer, err := newClaimBuffer(queue, false)
	if err != nil {
		t.Fatal(err)
	}
	token := uuid.NewV7()
	state := newRangeState(&controller.ClaimedRange{Lease: controller.RangeLease{Token: token}}, false)
	buffer.track(state)

	var winners atomic.Int32
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			if buffer.markStale(token) {
				winners.Add(1)
			}
		})
	}
	wg.Wait()
	if winners.Load() != 1 || !state.stale.Load() {
		t.Fatalf("warning winners = %d, stale = %v", winners.Load(), state.stale.Load())
	}

	buffer.remove(token)
	if buffer.markStale(token) {
		t.Fatal("removed range produced a warning")
	}
}
