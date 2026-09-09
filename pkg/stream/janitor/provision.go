package janitor

import (
	"context"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/controller"
)

// Declare writes the definition as the owner's worker row -- the newest
// declaration wins. Registers run it every time, so a declaration lost to a
// crash heals on the next one.
func (d *JanitorProvisioner) Declare(ctx context.Context, owner *common.Owner) error {
	return d.workers.DeclareWorker(ctx, d.definition, owner)
}

// Provision claims one live instance. nil = declined (target_instances
// already filled) -- not an error, try again later.
func (d *JanitorProvisioner) Provision(ctx context.Context, declared *worker.Worker) (worker.Execution, error) {
	// the owner is read before the claim (stream resolution below), so its check
	// cannot wait for RegisterInstance's
	if err := controller.ValidateOwner(declared.Owner, common.OwnerStream, WorkerStreamJanitor); err != nil {
		return nil, err
	}
	parsed, err := controller.ParseMetadata[janitorMetadata](declared.Metadata)
	if err != nil {
		return nil, err
	}
	if err := parsed.Validate(); err != nil {
		return nil, err
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
	claimed, err := d.workers.RegisterInstance(ctx, declared.Id, declared.Owner, common.OwnerStream, WorkerStreamJanitor, d.Config.InstanceTTL)
	if err != nil || claimed == nil {
		return nil, err
	}
	return newJanitorInstance(d, current, claimed, parsed)
}
