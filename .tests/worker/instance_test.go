package worker

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/worker/controller/datastore"
	"golang.org/x/sync/errgroup"
)

// invariant (one live lease): concurrent claims against a target of one
// insert exactly one worker_instance row.
func TestConcurrentClaimsAtTargetOneYieldOneInstance(t *testing.T) {
	// setup
	workers, owner := newWorkerDatastore(t)
	workerId := declareWorker(t, workers, owner, "collector", 1)

	// test
	claims := make(chan *datastore.WorkerInstanceRow, 16)
	var group errgroup.Group
	for range 16 {
		group.Go(func() error {
			instance, err := workers.ClaimInstance(t.Context(), workerId, time.Minute)
			claims <- instance
			return err
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
	close(claims)

	// verify
	claimed := 0
	for instance := range claims {
		if instance != nil {
			claimed++
		}
	}
	if claimed != 1 {
		t.Fatalf("rows claimed by 16 concurrent claimants = %d, want 1", claimed)
	}
}
