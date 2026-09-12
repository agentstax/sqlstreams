package vacuum

import (
	"context"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker/controller"
)

// Declare updates settings while preserving the existing operational target.
func (d *VacuumProvisioner) Declare(ctx context.Context, owner *common.Owner) error {
	return d.workers.DeclareWorker(ctx, d.definition, owner)
}

// Provision claims one live instance. nil = declined (target_instances
// already filled) -- not an error, try again later.
func (d *VacuumProvisioner) Provision(ctx context.Context, declared *worker.Worker) (worker.Execution, error) {
	// Stream resolution reads the owner before RegisterInstance validates it.
	if err := controller.ValidateOwner(declared.Owner, common.OwnerStream, WorkerStreamVacuum); err != nil {
		return nil, err
	}
	parsed, err := controller.ParseMetadata[vacuumMetadata](declared.Metadata)
	if err != nil {
		return nil, err
	}
	if err := parsed.Validate(); err != nil {
		return nil, err
	}

	if declared.TargetInstances == 0 {
		return nil, nil
	}
	if declared.TargetInstances != 1 {
		return nil, fmt.Errorf("target_instances must be 1 for stream_vacuum, got %v", declared.TargetInstances)
	}

	// stream resolution before the claim: a failure here leaves no claimed
	// instance behind to block reconciles until its TTL lapses
	current, err := d.streams.GetById(ctx, declared.Owner.StreamId)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, stream.ErrStreamNotFound.With("stream_id", declared.Owner.StreamId)
	}
	claimed, err := d.workers.RegisterInstance(ctx, declared.Id, declared.Owner, common.OwnerStream, WorkerStreamVacuum, d.Config.InstanceTTL)
	if err != nil || claimed == nil {
		return nil, err
	}
	return newVacuumInstance(d, current, claimed, parsed)
}
