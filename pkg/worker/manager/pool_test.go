package manager

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"golang.org/x/sync/errgroup"
)

func TestDeclinedClaimWaitsOutItsRetryDelay(t *testing.T) {
	// setup
	declining, err := newDecliningProvisioner("collector")
	if err != nil {
		t.Fatal(err)
	}
	group, ctx := errgroup.WithContext(t.Context())
	backoff, err := newClaimBackoff((&common.RetryPolicy{BaseDelay: time.Hour, MaxDelay: time.Hour}).WithDefaults(), 0)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := newInstancePool(map[string]worker.Provisioner{"collector": declining}, group, backoff, logging.NewDefaultLogger(io.Discard))
	if err != nil {
		t.Fatal(err)
	}
	owner, err := common.NewSystemOwner(1)
	if err != nil {
		t.Fatal(err)
	}
	row := &worker.Worker{Id: 1, Name: "collector", Owner: owner, TargetInstances: 1}

	// test
	firstErr := pool.reconcile(ctx, []*worker.Worker{row})
	afterFirst := declining.claims
	secondErr := pool.reconcile(ctx, []*worker.Worker{row})
	afterSecond := declining.claims
	removedErr := pool.reconcile(ctx, nil)
	returnedErr := pool.reconcile(ctx, []*worker.Worker{row})
	afterReturn := declining.claims
	waitErr := group.Wait()

	// verify
	if firstErr != nil || secondErr != nil || removedErr != nil || returnedErr != nil || waitErr != nil {
		t.Fatalf("reconcile passes, Wait = %v, %v, %v, %v, %v; want nil", firstErr, secondErr, removedErr, returnedErr, waitErr)
	}
	if afterFirst != 1 {
		t.Errorf("claims after the first reconcile = %d, want 1", afterFirst)
	}
	if afterSecond != 1 {
		t.Errorf("claims after a reconcile inside the retry delay = %d, want 1 (no retry)", afterSecond)
	}
	if afterReturn != 2 {
		t.Errorf("claims after the row left and returned = %d, want 2 (retry forgotten)", afterReturn)
	}
}

// ***************
// *** HELPERS ***
// ***************

type decliningProvisioner struct {
	definition *worker.Definition
	claims     int
}

func newDecliningProvisioner(name string) (*decliningProvisioner, error) {
	definition, err := worker.NewDefinition(name, common.OwnerAny, 1, struct{}{})
	if err != nil {
		return nil, err
	}
	return &decliningProvisioner{definition: definition}, nil
}

func (p *decliningProvisioner) Definition() *worker.Definition {
	return p.definition
}

func (p *decliningProvisioner) Provision(ctx context.Context, declared *worker.Worker) (worker.Execution, error) {
	p.claims++
	return nil, nil
}
